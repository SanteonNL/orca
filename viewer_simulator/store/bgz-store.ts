import { AllergyIntolerance, Condition, Consent, Coverage, Immunization, MedicationStatement, Observation, Patient, Procedure } from 'fhir/r4';
import { create } from 'zustand';

interface BgzState {
    patient?: Patient;
    allergyIntolerances: AllergyIntolerance[];
    conditions: Condition[];
    medicationStatements: MedicationStatement[];
    immunizations: Immunization[];
    procedures: Procedure[];
    coverages: Coverage[]
    consents: Consent[];
    observations: Observation[];
    loaded: boolean;
    setBgzData: (data: Partial<BgzState>) => void;
    setLoaded: (loaded: boolean) => void;
    clearBgzData: () => void;
}

const useBgzStore = create<BgzState>((set) => ({
    patient: undefined,
    allergyIntolerances: [],
    conditions: [],
    medicationStatements: [],
    immunizations: [],
    procedures: [],
    coverages: [],
    consents: [],
    observations: [],
    loaded: false,
    setBgzData: (data) => set((state) => ({ ...state, ...data })),
    setLoaded: (loaded) => set({ loaded }),
    clearBgzData: () => set({
        patient: undefined,
        allergyIntolerances: [],
        conditions: [],
        medicationStatements: [],
        immunizations: [],
        procedures: [],
        coverages: [],
        consents: [],
        observations: [],
        loaded: false,
    }),
}));

export default useBgzStore;