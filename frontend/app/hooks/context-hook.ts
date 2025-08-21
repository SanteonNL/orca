import React from "react";
import { useQuery } from "@tanstack/react-query";
import { createCpsClient, createEhrClient, createScpClient } from "@/lib/fhirUtils";
import Client from "fhir-kit-client";

export interface LaunchContext {
    patient: string
    practitioner: string
    practitionerRole: string
    tenantId: string
    serviceRequest?: string
    task?: string
    taskIdentifier?: string
}

export interface ContextHookResult {
    launchContext?: LaunchContext;
    ehrClient?: Client;
    cpsClient?: Client;
    scpClient?: Client;
    isLoading: boolean;
    isError: boolean;
    error?: Error | null;
}

const fetchLaunchContext = async (): Promise<LaunchContext> => {
    const launchContextRes = await fetch(`/orca/cpc/context`);
    if (!launchContextRes.ok) {
        throw new Error(`Failed to fetch context: ${launchContextRes.statusText}`);
    }
    return await launchContextRes.json();
};

export default function useContext(): ContextHookResult {
    const { data: launchContext, isLoading, isError, error } = useQuery({
        queryKey: ['launch-context'],
        queryFn: fetchLaunchContext,
        staleTime: 60 * 60 * 1000, // 1 hour - launch context rarely changes
        retry: 3,
    });

    // Create FHIR clients based on launch context
    const clients = React.useMemo(() => {
        if (!launchContext?.tenantId) {
            return {
                ehrClient: undefined,
                cpsClient: undefined,
                scpClient: undefined,
            };
        }

        return {
            ehrClient: createEhrClient(launchContext.tenantId),
            cpsClient: createCpsClient(launchContext.tenantId),
            scpClient: createScpClient(launchContext.tenantId),
        };
    }, [launchContext?.tenantId]);

    return {
        launchContext,
        ...clients,
        isLoading,
        isError,
        error,
    };
}