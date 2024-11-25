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
    setBgzData: (data: Partial<BgzState>) => void;
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
    setBgzData: (data) => set((state) => ({ ...state, ...data })),
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
    }),
}));

export default useBgzStore;