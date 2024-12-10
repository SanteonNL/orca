import { ProcedureRequest } from 'fhir/r3';
import { DeviceUseStatement, Condition, Consent, Coverage, Encounter, Flag, Immunization, MedicationRequest, NutritionOrder, Observation, Patient, Procedure, ImmunizationRecommendation, Appointment, DeviceRequest } from 'fhir/r3';
import { create } from 'zustand';

interface BgzState {
    patient?: Patient;
    appointments: Appointment[];
    conditions: Condition[];
    coverages: Coverage[];
    consents: Consent[];
    observations: Observation[];
    immunizations: Immunization[];
    immunizationRecommendations: ImmunizationRecommendation[];
    deviceRequests: DeviceRequest[];
    deviceUseStatements: DeviceUseStatement[];
    encounters: Encounter[];
    flags: Flag[];
    medicationRequests: MedicationRequest[];
    nutritionOrders: NutritionOrder[];
    procedures: Procedure[];
    procedureRequests: ProcedureRequest[]; //TODO: Remove? STU3?
    loaded: boolean;
    addBgzData: (data: Partial<BgzState>) => void;
    setBgzData: (data: BgzState) => void;
    setLoaded: (loaded: boolean) => void;
    clearBgzData: () => void;
}

const useBgzStore = create<BgzState>((set) => ({
    patient: undefined,
    appointments: [],
    conditions: [],
    coverages: [],
    consents: [],
    observations: [],
    immunizations: [],
    immunizationRecommendations: [],
    deviceRequests: [],
    deviceUseStatements: [],
    devices: [],
    deviceUses: [],
    encounters: [],
    flags: [],
    medicationRequests: [],
    nutritionOrders: [],
    procedures: [],
    procedureRequests: [],
    loaded: false,
    addBgzData: (data) => {
        set((state) => ({ ...state, ...data }))
    },
    setBgzData: (data) => set(data),
    setLoaded: (loaded) => set({ loaded }),
    clearBgzData: () => set({
        patient: undefined,
        appointments: [],
        conditions: [],
        coverages: [],
        consents: [],
        observations: [],
        immunizations: [],
        immunizationRecommendations: [],
        deviceRequests: [],
        deviceUseStatements: [],
        encounters: [],
        flags: [],
        medicationRequests: [],
        nutritionOrders: [],
        procedures: [],
        procedureRequests: [],
        loaded: false,
    }),
}));

export default useBgzStore;