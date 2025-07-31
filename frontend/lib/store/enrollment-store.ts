import { CarePlan, Condition, Patient, Practitioner, PractitionerRole, ServiceRequest } from 'fhir/r4';
import { useEffect } from 'react';
import { create } from 'zustand';
import useContext, {LaunchContext, useContextStore} from "@/lib/store/context-store";
import Client from "fhir-kit-client";

interface StoreState {
    initialized: boolean
    patient?: Patient
    practitioner?: Practitioner
    practitionerRole?: PractitionerRole
    serviceRequest?: ServiceRequest
    selectedCarePlan?: CarePlan | null
    taskCondition?: Condition
    loading: boolean
    error?: string
    setSelectedCarePlan: (carePlan?: CarePlan) => void
    setTaskCondition: (condition?: Condition) => void
    fetchAllResources: () => Promise<void>
}

// Define the Zustand store
const useEnrollmentStore = create<StoreState>((set, get) => ({
    initialized: false,
    launchContext: undefined,
    patient: undefined,
    practitioner: undefined,
    practitionerRole: undefined,
    serviceRequest: undefined,
    selectedCarePlan: undefined,
    taskCondition: undefined,
    loading: false,
    error: undefined,
    setSelectedCarePlan: (carePlan?: CarePlan) => {
        set({ selectedCarePlan: carePlan });
    },
    setTaskCondition: (condition?: Condition) => {
        set({ taskCondition: condition });
    },
    fetchAllResources: async () => {
        try {
            const { loading } = get()

            if (!loading) {
                set({ loading: true, error: undefined })

                const contextState = useContextStore.getState();
                if (contextState.launchContext && contextState.ehrClient) {
                    await fetchEhrResources(contextState.launchContext, contextState.ehrClient, get, set);
                    set({ initialized: true, loading: false });
                } else {
                    set({ loading: false });
                }
            }

        } catch (error: any) {
            set({ error: `Something went wrong while fetching all resources: ${error?.message || error}`, loading: false })
        }
    },
}));

const fetchEhrResources = async (launchContext: LaunchContext, ehrClient: Client, get: () => StoreState, set: (partial: StoreState | Partial<StoreState> | ((state: StoreState) => StoreState | Partial<StoreState>), replace?: false | undefined) => void) => {
    if (typeof window === "undefined") return //skip during build

    const [patient, practitioner, practitionerRole, serviceRequest] = await Promise.all([
        ehrClient.read({ resourceType: 'Patient', id: launchContext.patient.replace("Patient/", "") }),
        ehrClient.read({ resourceType: 'Practitioner', id: launchContext.practitioner.replace("Practitioner/", "") }),
        launchContext.practitionerRole
            ? ehrClient.read({ resourceType: 'PractitionerRole', id: launchContext.practitionerRole.replace("PractitionerRole/", "") })
            : Promise.resolve(undefined as PractitionerRole | undefined),
        launchContext.serviceRequest
            ? ehrClient.read({ resourceType: 'ServiceRequest', id: launchContext.serviceRequest.replace("ServiceRequest/", "") })
            : Promise.resolve(undefined as ServiceRequest | undefined)
    ]);

    const sr = serviceRequest as ServiceRequest

    //We extract the Task Condition from the ServiceRequest, for now, simply match the first Condition reference
    //TODO: We need to ensure only one Condition is bound to the ServiceRequest
    const taskReference = sr.reasonReference?.find(ref => ref.reference?.startsWith("Condition"))

    if (taskReference && taskReference.reference) {
        const taskCondition = await ehrClient.read({ resourceType: 'Condition', id: taskReference.reference.replace("Condition/", "") }) as Condition
        set({ taskCondition });
    } else {
        console.warn(`No Task Condition found for ServiceRequest/${serviceRequest?.id ?? "(missing)"}`);
    }

    set({
        patient: patient as Patient,
        practitioner: practitioner as Practitioner,
        practitionerRole: practitionerRole as PractitionerRole,
        serviceRequest: sr,
    });

    return patient;
};

const useEnrollment = () => {
    const {cpsClient} = useContext()
    const store = useEnrollmentStore();
    const initialized = useEnrollmentStore(state => state.initialized);
    const fetchAllResources = useEnrollmentStore(state => state.fetchAllResources);

    useEffect(() => {
        if (!initialized) {
            fetchAllResources();
        }
    }, [fetchAllResources, initialized, cpsClient]);

    return store;
};

export default useEnrollment;
