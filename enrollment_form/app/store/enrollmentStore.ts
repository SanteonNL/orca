import { Patient, Practitioner, ServiceRequest } from 'fhir/r4';
import { create } from 'zustand';

interface StoreState {
    patient?: Patient
    practitioner?: Practitioner
    serviceRequest?: ServiceRequest
    loading: boolean
    error?: string
    fetchPatient: () => Promise<void>
    fetchPractitioner: () => Promise<void>
    fetchServiceRequest: () => Promise<void>
}

// Define the Zustand store
const useEnrollmentStore = create<StoreState>((set) => ({
    patient: undefined,
    practitioner: undefined,
    serviceRequest: undefined,
    loading: false,
    error: undefined,

    fetchPatient: async () => {
        set({ loading: true, error: undefined });
        try {
            const response = await fetch('/orca/contrib/patient');
            // const response = await fetch('http://localhost:9090/fhir/Patient/3');

            if (!response.ok) {
                set({ patient: undefined, error: response.statusText, loading: false });
            } else {
                const data = await response.json();
                set({ patient: data, loading: false });
            }
        } catch (error) {
            set({ error: "Failed to fetch patient", loading: false });
        }
    },

    fetchPractitioner: async () => {
        set({ loading: true, error: undefined });
        try {
            const response = await fetch('/orca/contrib/practitioner');
            // const response = await fetch(`http://localhost:9090/fhir/Practitioner/6`);

            if (!response.ok) {
                set({ practitioner: undefined, error: response.statusText, loading: false });
            } else {
                const data = await response.json();
                set({ practitioner: data, loading: false });
            }
        } catch (error) {
            set({ error: "Failed to fetch practitioner", loading: false });
        }
    },
    fetchServiceRequest: async () => {
        set({ loading: true, error: undefined });
        try {
            const response = await fetch('/orca/contrib/serviceRequest');
            // const response = await fetch('http://localhost:9090/fhir/ServiceRequest/5');

            if (!response.ok) {
                set({ serviceRequest: undefined, error: response.statusText, loading: false });
            } else {
                const data = await response.json();
                set({ serviceRequest: data, loading: false });
            }

        } catch (error) {
            set({ error: "Failed to fetch service request", loading: false });
        }
    },
}));

export default useEnrollmentStore;
