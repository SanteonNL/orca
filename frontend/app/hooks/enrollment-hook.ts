import { useClients, useLaunchContext } from "@/app/hooks/context-hook";
import {useQuery} from "@tanstack/react-query";
import {Condition, Patient, Practitioner, PractitionerRole, ServiceRequest} from "fhir/r4";

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

export default function useEnrollment(): EnrollmentHookResult {
  const { launchContext } = useLaunchContext()
  const { ehrClient } = useClients()

  const enabled = !!launchContext && !!ehrClient
  const staleTime = 5 * 60 * 1000; // 5 minutes - enrollment resources don't change frequently

  const patientId = launchContext?.patient.replace("Patient/", "");
  const patientQuery = useQuery({
    queryKey: ['patient', patientId],
    queryFn: () => ehrClient!.read({ resourceType: 'Patient', id: patientId! }) as Promise<Patient>,
    enabled: enabled,
    staleTime
  })

  const practitionerId = launchContext?.practitioner.replace("Practitioner/", "");
  const practitionerQuery = useQuery({
    queryKey: ['practitioner', practitionerId],
    queryFn: () => ehrClient!.read({ resourceType: 'Practitioner', id: practitionerId! }) as Promise<Practitioner>,
    enabled: enabled,
    staleTime
  })

  const practitionerRoleId = launchContext?.practitionerRole?.replace("PractitionerRole/", "");
  const practitionerRoleQuery = useQuery({
    queryKey: ['practitionerRole', practitionerRoleId],
    queryFn: () => ehrClient!.read({ resourceType: 'PractitionerRole', id: practitionerRoleId! }) as Promise<PractitionerRole>,
    enabled: enabled && !!practitionerRoleId,
    staleTime
  })

  const serviceRequestId = launchContext?.serviceRequest?.replace("ServiceRequest/", "");
  const serviceRequestQuery = useQuery({
    queryKey: ['serviceRequest', serviceRequestId],  
    queryFn: () => ehrClient!.read({ resourceType: 'ServiceRequest', id: serviceRequestId! }) as Promise<ServiceRequest>,
    enabled: enabled && !!serviceRequestId,
    staleTime
  })

  // Extract the Task Condition from the ServiceRequest, for now, simply match the first Condition reference
  // TODO: We need to ensure only one Condition is bound to the ServiceRequest
  const taskReference = serviceRequestQuery.data?.reasonReference?.find(ref => ref.reference?.startsWith("Condition"))
  const conditionId = taskReference?.reference?.replace("Condition/", "");

  const conditionQuery = useQuery({
    queryKey: ['task-condition', {serviceRequestId, conditionId}],
    queryFn: () => {
      if (!conditionId) {
        console.warn(`No Task Condition found for ServiceRequest/${serviceRequestId ?? "(missing)"}`)
        return null
      }
      else return ehrClient!.read({ resourceType: 'Condition', id: conditionId! }) as Promise<Condition>
    },
    enabled: enabled && serviceRequestQuery.isSuccess,
    staleTime
  })

  const isLoading = patientQuery.isLoading || practitionerQuery.isLoading || practitionerRoleQuery.isLoading || serviceRequestQuery.isLoading || conditionQuery.isLoading
  const isError = patientQuery.isError || practitionerQuery.isError || practitionerRoleQuery.isError || serviceRequestQuery.isError || conditionQuery.isError
  const error = patientQuery.error || practitionerQuery.error || practitionerRoleQuery.error || serviceRequestQuery.error || conditionQuery.error

  return {
    patient: patientQuery.data,
    practitioner: practitionerQuery.data,
    practitionerRole: practitionerRoleQuery.data,
    serviceRequest: serviceRequestQuery.data,
    taskCondition: conditionQuery.data ?? undefined,
    isLoading,
    isError,
    error
  }
}