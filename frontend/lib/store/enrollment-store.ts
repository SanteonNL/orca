import { Bundle, CarePlan, Condition, Patient, Practitioner, ServiceRequest } from 'fhir/r4';
import { useEffect } from 'react';
import { create } from 'zustand';
import { BSN_SYSTEM, createCpsClient, createEhrClient, fetchAllBundlePages, getBsn } from '../fhirUtils';

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
    serviceRequest: undefined,
    carePlans: undefined,
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

                await fetchLaunchContext(set);
                await fetchEhrResources(get, set);
                await fetchCarePlans(get, set);

                set({ initialized: true, loading: false });
            }

        } catch (error: any) {
            set({ error: `Something went wrong while fetching all resources: ${error?.message || error}`, loading: false })
        }
    },
}));

const fetchLaunchContext = async (set: (partial: StoreState | Partial<StoreState> | ((state: StoreState) => StoreState | Partial<StoreState>), replace?: boolean | undefined) => void) => {

    let launchContext: LaunchContext;

    if (process.env.NODE_ENV === "production") {
        const launchContextRes = await fetch(`/orca/cpc/context`);
        if (!launchContextRes.ok) throw new Error(`Failed to fetch patient: ${launchContextRes.statusText}`);

        launchContext = await launchContextRes.json();
    } else {
        //TODO: We can remove this when going live, this is useful during development
        launchContext = {
            "patient": "Patient/2",
            "serviceRequest": "ServiceRequest/4",
            "practitioner": "Practitioner/7"
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

    const [patient, practitioner, serviceRequest] = await Promise.all([
        ehrClient.read({ resourceType: 'Patient', id: launchContext.patient.replace("Patient/", "") }),
        ehrClient.read({ resourceType: 'Practitioner', id: launchContext.practitioner.replace("Practitioner/", "") }),
        ehrClient.read({ resourceType: 'ServiceRequest', id: launchContext.serviceRequest.replace("ServiceRequest/", "") }),
    ]);

    const sr = serviceRequest as ServiceRequest

    //We extract the Task Condition from the ServiceRequest, for now, simply match the first Condition reference
    //TODO: We need to ensure only one Condition is bound to the ServiceRequest
    const taskReference = sr.reasonReference?.find(ref => ref.reference?.startsWith("Condition"))

    if (taskReference && taskReference.reference) {
        const taskCondition = await ehrClient.read({ resourceType: 'Condition', id: taskReference.reference.replace("Condition/", "") }) as Condition
        set({ taskCondition });
    } else {
        console.warn(`No Task Condition found for ServiceRequest/${serviceRequest.id}`)
    }

    set({
        patient: patient as Patient,
        practitioner: practitioner as Practitioner,
        serviceRequest: sr,
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
    if (!bsn) throw new Error(`No BSN identifier found for Patient / ${patient?.id}`);

    const cpsClient = createCpsClient()

    const initialBundle = await cpsClient.search({
        resourceType: 'CarePlan',
        searchParams: {
            'subject-identifier': `${BSN_SYSTEM}| ${bsn} `,
            '_count': 100
        }
    }) as Bundle<CarePlan>;

    const carePlans = await fetchAllBundlePages(cpsClient, initialBundle);
    set({ carePlans });
};

const useEnrollment = () => {
    const store = useEnrollmentStore();
    const initialized = useEnrollmentStore(state => state.initialized);
    const fetchAllResources = useEnrollmentStore(state => state.fetchAllResources);

    useEffect(() => {
        if (!initialized) {
            fetchAllResources();
        }
    }, []);

    return store;
};

export default useEnrollment;
