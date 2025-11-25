import {useQuery, useMutation, useQueryClient} from "@tanstack/react-query";
import {CarePlan, Condition, Patient, Practitioner, PractitionerRole, ServiceRequest} from "fhir/r4";
import {
  LaunchContext,
  useClients,
  useLaunchContext,
} from '@/app/hooks/context-hook'
import Client from "fhir-kit-client";

export type EnrollmentResources = {
    patient: Patient;
    practitioner: Practitioner;
    practitionerRole?: PractitionerRole;
    serviceRequest?: ServiceRequest;
    taskCondition?: Condition;
};

type EnrollmentHookResult = {
    patient?: Patient;
    practitioner?: Practitioner;
    practitionerRole?: PractitionerRole;
    serviceRequest?: ServiceRequest;
    taskCondition?: Condition;
    isLoading: boolean;
    isError: boolean;
    error?: Error | null;
}

const fetchEhrResources = async (launchContext: LaunchContext, ehrClient: Client): Promise<EnrollmentResources> => {
    const [patient, practitioner, practitionerRole, serviceRequest] = await Promise.all([
        ehrClient.read({resourceType: 'Patient', id: launchContext.patient.replace("Patient/", "")}),
        ehrClient.read({resourceType: 'Practitioner', id: launchContext.practitioner.replace("Practitioner/", "")}),
        launchContext.practitionerRole
            ? ehrClient.read({
                resourceType: 'PractitionerRole',
                id: launchContext.practitionerRole.replace("PractitionerRole/", "")
            })
            : Promise.resolve(undefined as PractitionerRole | undefined),
        launchContext.serviceRequest
            ? ehrClient.read({
                resourceType: 'ServiceRequest',
                id: launchContext.serviceRequest.replace("ServiceRequest/", "")
            })
            : Promise.resolve(undefined as ServiceRequest | undefined)
    ]);

    const sr = serviceRequest as ServiceRequest

    // Extract the Task Condition from the ServiceRequest, for now, simply match the first Condition reference
    // TODO: We need to ensure only one Condition is bound to the ServiceRequest
    const taskReference = sr?.reasonReference?.find(ref => ref.reference?.startsWith("Condition"))

    let taskCondition: Condition | undefined = undefined;
    if (taskReference && taskReference.reference) {
        taskCondition = await ehrClient.read({
            resourceType: 'Condition',
            id: taskReference.reference.replace("Condition/", "")
        }) as Condition
    } else {
        console.warn(`No Task Condition found for ServiceRequest/${serviceRequest?.id ?? "(missing)"}`);
    }

    return {
        patient: patient as Patient,
        practitioner: practitioner as Practitioner,
        practitionerRole: practitionerRole as PractitionerRole,
        serviceRequest: sr,
        taskCondition: taskCondition,
    };
};

export default function useEnrollment(): EnrollmentHookResult {
    const { launchContext } = useLaunchContext()
    const { ehrClient } = useClients()

    const {data, isLoading, isError, error} = useQuery({
        queryKey: ['enrollment-resources', launchContext?.patient, launchContext?.practitioner],
        queryFn: () => fetchEhrResources(launchContext!, ehrClient!),
        enabled: !!launchContext && !!ehrClient,
        staleTime: 5 * 60 * 1000, // 5 minutes - enrollment resources don't change frequently
    });

    return {
        patient: data?.patient,
        practitioner: data?.practitioner,
        practitionerRole: data?.practitionerRole,
        serviceRequest: data?.serviceRequest,
        taskCondition: data?.taskCondition,
        isLoading,
        isError,
        error
    };
}