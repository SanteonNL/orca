import Client from 'fhir-kit-client';
import { Bundle, CarePlan, Condition, Patient, Practitioner, ServiceRequest } from 'fhir/r4';
import { useEffect } from 'react';
import { create } from 'zustand';
import { fetchAllBundlePages, getBsn } from '../fhirUtils';

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
    conditions?: Condition[]
    primaryCondition?: Condition
    relevantConditions?: Condition[]
    shouldCreateNewCarePlan: boolean
    loading: boolean
    error?: string
    setSelectedCarePlan: (carePlan: CarePlan) => void
    setPrimaryCondition: (condition?: Condition) => void
    addRelevantCondition: (condition: Condition) => void
    removeRelevantCondition: (condition: Condition) => void
    setRelevantConditions: (conditions: Condition[]) => void
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
    conditions: undefined,
    primaryCondition: undefined,
    relevantConditions: undefined,
    shouldCreateNewCarePlan: false,
    loading: false,
    error: undefined,
    setSelectedCarePlan: (carePlan: CarePlan) => {
        set({ selectedCarePlan: carePlan });
    },
    setPrimaryCondition: (condition?: Condition) => {
        set({ primaryCondition: condition });
    },
    addRelevantCondition: (condition: Condition) => {
        set((state) => ({
            relevantConditions: [...state.relevantConditions || [], condition]
        }))
    },
    removeRelevantCondition: (condition: Condition) => {
        set((state) => ({
            relevantConditions: state.relevantConditions?.filter(filterCondition => filterCondition.id !== condition.id)
        }))
    },
    setRelevantConditions: (conditions: Condition[]) => {
        set({
            relevantConditions: conditions
        })
    },
    setShouldCreateNewCarePlan: (createNewCarePlan: boolean) => {
        set({ shouldCreateNewCarePlan: createNewCarePlan })
    },
    fetchAllResources: async () => {

        try {
            set({ loading: true, error: undefined })

            await fetchLaunchContext(set);
            await fetchEhrResources(get, set);
            await fetchCarePlans(get, set);

            set({ initialized: true, loading: false, })

        } catch (error: any) {
            set({ error: `Something went wrong while fetching all resources: ${error?.message || error}`, loading: false })
        }
    },
}));

const fetchLaunchContext = async (set: (partial: StoreState | Partial<StoreState> | ((state: StoreState) => StoreState | Partial<StoreState>), replace?: boolean | undefined) => void) => {
    const launchContextRes = await fetch(`/orca/contrib/context`);
    if (!launchContextRes.ok) throw new Error(`Failed to fetch patient: ${launchContextRes.statusText}`);

    const launchContext = await launchContextRes.json();
    // Use below for quick localhost testing
    // const launchContext = {
    //     "patient": "Patient/2",
    //     "serviceRequest": "ServiceRequest/4",
    //     "practitioner": "Practitioner/6"
    // }

    set({ launchContext });

    return launchContext;
};

const fetchEhrResources = async (get: () => StoreState, set: (partial: StoreState | Partial<StoreState> | ((state: StoreState) => StoreState | Partial<StoreState>), replace?: boolean | undefined) => void) => {
    const { launchContext } = get();

    if (!launchContext) throw new Error("Unable to fetch EHR resources without LaunchContext")

    if (typeof window === "undefined") return //skip during build

    const client = new Client({ baseUrl: `${window && window.location.origin}/orca/contrib/ehr/fhir` });
    // const client = new Client({ baseUrl: `http://localhost:9090/fhir` });

    const [patient, practitioner, serviceRequest, conditions] = await Promise.all([
        client.read({ resourceType: 'Patient', id: launchContext.patient.replace("Patient/", "") }),
        client.read({ resourceType: 'Practitioner', id: launchContext.practitioner.replace("Practitioner/", "") }),
        client.read({ resourceType: 'ServiceRequest', id: launchContext.serviceRequest.replace("ServiceRequest/", "") }),
        client.search({ resourceType: 'Condition', searchParams: { patient: launchContext.patient, "_count": 100 } }),
    ]);

    const allConditions = await fetchAllBundlePages(client, conditions as Bundle<Condition>); //paginate in case there are more than 100 conditions or the server doesn't allow for a large pagination size

    set({
        patient: patient as Patient,
        practitioner: practitioner as Practitioner,
        serviceRequest: serviceRequest as ServiceRequest,
        conditions: allConditions
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

    const client = new Client({ baseUrl: `${window.location.origin}/orca/contrib/cps/fhir` });
    // const client = new Client({ baseUrl: `http://localhost:7090/fhir` });

    const initialBundle = await client.search({
        resourceType: 'CarePlan',
        searchParams: {
            //TODO: FIXME, cannot search on CarePlan.patient.identifier and the Patient is not stored on the CPS
            //Follow discission @ https://chat.fhir.org/#narrow/stream/179166-implementers/topic/Searching.20by.20CarePlan.2Epatient.2Eidentifier
            // 'patient.identifier': `http://fhir.nl/fhir/NamingSystem/bsn|${bsn}`,
            '_count': 100
        }
    }) as Bundle<CarePlan>;

    const carePlans = await fetchAllBundlePages(client, initialBundle);
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
