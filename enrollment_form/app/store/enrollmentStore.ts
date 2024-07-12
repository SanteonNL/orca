import { Patient, ServiceRequest } from 'fhir/r4';
import { create } from 'zustand';

interface StoreState {
    patient?: Patient
    serviceRequest?: ServiceRequest
    loading: boolean
    error?: string
    fetchPatient: () => Promise<void>
    fetchServiceRequest: () => Promise<void>
}

// Define the Zustand store
const useEnrollmentStore = create<StoreState>((set) => ({
    patient: undefined,
    serviceRequest: undefined,
    loading: false,
    error: undefined,

    fetchPatient: async () => {
        set({ loading: true, error: undefined });
        try {
            const response = await fetch('/orca/contrib/patient');

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

    fetchServiceRequest: async () => {
        set({ loading: true, error: undefined });
        try {
            const response = await fetch('/orca/contrib/serviceRequest');

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
