import Client from 'fhir-kit-client';
import { Bundle, CarePlan, Condition, Patient, Practitioner, ServiceRequest } from 'fhir/r4';
import { useEffect } from 'react';
import { create } from 'zustand';
import { BSN_SYSTEM, createCpsClient, createEhrClient, fetchAllBundlePages, getBsn } from '../fhirUtils';
import useEhrFhirClient from '@/hooks/use-ehr-fhir-client';

interface LaunchContext {
    patient: string
    practitioner: string
    serviceRequest: string
}

interface StoreState {
    initialized: boolean
    launchContext?: LaunchContext
    patient?: Patient
    practitioner?: Practitioner
    serviceRequest?: ServiceRequest
    carePlans?: CarePlan[]
    selectedCarePlan?: CarePlan | null
    newCarePlanName: string
    taskCondition?: Condition
    patientConditions?: Condition[]
    carePlanConditions?: Condition[]
    shouldCreateNewCarePlan: boolean
    loading: boolean
    error?: string
    setSelectedCarePlan: (carePlan?: CarePlan) => void
    setNewCarePlanName: (name: string) => void
    setTaskCondition: (condition?: Condition) => void
    setPatientConditions: (conditions: Condition[]) => void
    setCarePlanConditions: (conditions: Condition[]) => void
    setShouldCreateNewCarePlan: (createNewCarePlan: boolean) => void
    fetchAllResources: () => Promise<void>
}

// Define the Zustand store
const useEnrollmentStore = create<StoreState>((set, get) => ({
    initialized: false,
    launchContext: undefined,
    patient: undefined,
    practitioner: undefined,
    serviceRequest: undefined,
    carePlans: undefined,
    selectedCarePlan: undefined,
    newCarePlanName: "",
    taskCondition: undefined,
    patientConditions: undefined,
    shouldCreateNewCarePlan: false,
    loading: false,
    error: undefined,
    setSelectedCarePlan: (carePlan?: CarePlan) => {
        set({ selectedCarePlan: carePlan });

        if (!carePlan) {
            set({ carePlanConditions: undefined, taskCondition: undefined })
        } else {

            const { patientConditions } = get();
            const carePlanConditions = carePlan.addresses?.map(conditionCode =>
                patientConditions?.find(patientCondition => patientCondition.code?.coding?.find(coding =>
                    coding.system === conditionCode.identifier?.system &&
                    coding.code === conditionCode.identifier?.value
                ))
            ).filter(condition => condition !== undefined)

            set({ carePlanConditions, taskCondition: undefined })
        }
    },
    setNewCarePlanName: (name: string) => {
        set({ newCarePlanName: name });
    },
    setTaskCondition: (condition?: Condition) => {
        set({ taskCondition: condition });
    },
    setPatientConditions: (conditions: Condition[]) => {
        set({ patientConditions: conditions })
    },
    setCarePlanConditions: (conditions: Condition[]) => {
        set({ carePlanConditions: conditions })
    },
    setShouldCreateNewCarePlan: (createNewCarePlan: boolean) => {
        set({ shouldCreateNewCarePlan: createNewCarePlan })
        if (createNewCarePlan) set({ selectedCarePlan: undefined })
    },
    fetchAllResources: async () => {

        try {
            const { loading } = get()

            if (!loading) {
                set({ loading: true, error: undefined })

                await fetchLaunchContext(set);
                await fetchEhrResources(get, set);
                await fetchCarePlans(get, set);

                set({ initialized: true, loading: false, })
            }

        } catch (error: any) {
            set({ error: `Something went wrong while fetching all resources: ${error?.message || error}`, loading: false })
        }
    },
}));

const fetchLaunchContext = async (set: (partial: StoreState | Partial<StoreState> | ((state: StoreState) => StoreState | Partial<StoreState>), replace?: boolean | undefined) => void) => {

    let launchContext: LaunchContext;

    if (process.env.NODE_ENV === "production") {
        const launchContextRes = await fetch(`/orca/contrib/context`);
        if (!launchContextRes.ok) throw new Error(`Failed to fetch patient: ${launchContextRes.statusText}`);

        launchContext = await launchContextRes.json();
    } else {
        //TODO: We can remove this when going live, this is useful during development
        launchContext = {
            "patient": "Patient/2",
            "serviceRequest": "ServiceRequest/4",
            "practitioner": "Practitioner/6"
        }

    }

    set({ launchContext });

    return launchContext;
};

const fetchEhrResources = async (get: () => StoreState, set: (partial: StoreState | Partial<StoreState> | ((state: StoreState) => StoreState | Partial<StoreState>), replace?: boolean | undefined) => void) => {
    const { launchContext } = get();

    if (!launchContext) throw new Error("Unable to fetch EHR resources without LaunchContext")

    if (typeof window === "undefined") return //skip during build

    const ehrClient = createEhrClient()

    const [patient, practitioner, serviceRequest, conditions] = await Promise.all([
        ehrClient.read({ resourceType: 'Patient', id: launchContext.patient.replace("Patient/", "") }),
        ehrClient.read({ resourceType: 'Practitioner', id: launchContext.practitioner.replace("Practitioner/", "") }),
        ehrClient.read({ resourceType: 'ServiceRequest', id: launchContext.serviceRequest.replace("ServiceRequest/", "") }),
        ehrClient.search({ resourceType: 'Condition', searchParams: { patient: launchContext.patient, "_count": 100 } }),
    ]);

    const allConditions = await fetchAllBundlePages(ehrClient, conditions as Bundle<Condition>); //paginate in case there are more than 100 conditions or the server doesn't allow for a large pagination size

    set({
        patient: patient as Patient,
        practitioner: practitioner as Practitioner,
        serviceRequest: serviceRequest as ServiceRequest,
        patientConditions: allConditions
    });

    return patient;
};

const fetchCarePlans = async (
    get: () => StoreState,
    set: (
        partial: StoreState | Partial<StoreState> | ((state: StoreState) => StoreState | Partial<StoreState>),
        replace?: boolean | undefined
    ) => void
) => {
    const { patient } = get();

    const bsn = getBsn(patient)
    if (!bsn) throw new Error(`No BSN identifier found for Patient/${patient?.id}`);

    const cpsClient = createCpsClient()

    const initialBundle = await cpsClient.search({
        resourceType: 'CarePlan',
        searchParams: {
            'subject-identifier': `${BSN_SYSTEM}|${bsn}`,
            '_count': 100
        }
    }) as Bundle<CarePlan>;

    const carePlans = await fetchAllBundlePages(cpsClient, initialBundle);
    set({ carePlans, initialized: true, loading: false });
};

const useEnrollment = () => {
    const store = useEnrollmentStore();

    useEffect(() => {
        if (!store.initialized) {
            store.fetchAllResources();
        }
    }, []);

    return store;
};

export default useEnrollment;
