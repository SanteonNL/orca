import { useEffect } from 'react';
import { create } from 'zustand';
import {createCpsClient, createEhrClient, createScpClient} from '../fhirUtils';
import Client from "fhir-kit-client";

export interface LaunchContext {
    patient: string
    practitioner: string
    practitionerRole: string
    serviceRequest: string
    tenantId: string
    task?: string
    taskIdentifier?: string
}

interface StoreState {
    launchContext?: LaunchContext
    error?: string
    ehrClient?: Client;
    cpsClient?: Client,
    scpClient?: Client,
    fetchContext: () => Promise<void>
}

// Define the Zustand store
export const useContextStore = create<StoreState>((set, get) => ({
    launchContext: undefined,
    error: undefined,
    cpsClient: undefined,
    ehrClient: undefined,
    scpClient: undefined,
    fetchContext: async () => {
        try {
            await fetchLaunchContext(set);
            const launchContext = get().launchContext;
            if (!launchContext) {
                set({ error: `Launch context is not available.` });
            }
            set({ cpsClient: createCpsClient(launchContext!.tenantId) });
            set({ ehrClient: createEhrClient(launchContext!.tenantId) });
            set({ scpClient: createScpClient(launchContext!.tenantId) });
        } catch (error: any) {
            set({ error: `Something went wrong while fetching the context: ${error?.message || error}`})
        }
    },
}));

const fetchLaunchContext = async (set: (partial: StoreState | Partial<StoreState> | ((state: StoreState) => StoreState | Partial<StoreState>), replace?: false | undefined) => void) => {
    let launchContext: LaunchContext;
    const launchContextRes = await fetch(`/orca/cpc/context`);
    if (!launchContextRes.ok) throw new Error(`Failed to fetch patient: ${launchContextRes.statusText}`);
    launchContext = await launchContextRes.json();
    set({ launchContext });
    return launchContext;
};


const useContext = () => {
    const store = useContextStore();
    const fetchAllResources = useContextStore(state => state.fetchContext);

    useEffect(() => {
        if (!store.launchContext) {
            fetchAllResources();
        }
    }, [fetchAllResources, store]);

    return store;
};

export default useContext;
