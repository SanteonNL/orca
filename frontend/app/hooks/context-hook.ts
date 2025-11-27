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

export interface LaunchContextHookResult {
  launchContext?: LaunchContext
  isLoading: boolean
  isError: boolean
  error?: Error | null
}

export interface ClientHookResult {
  ehrClient?: Client
  cpsClient?: Client
  scpClient?: Client
}

const fetchLaunchContext = async (): Promise<LaunchContext> => {
  const launchContextRes = await fetch(`/orca/cpc/context`)
  if (!launchContextRes.ok) {
    throw new Error(`Failed to fetch context: ${launchContextRes.statusText}`)
  }
  return await launchContextRes.json()
}

export function useLaunchContext(): LaunchContextHookResult {
  const {
    data: launchContext,
    isLoading,
    isError,
    error,
  } = useQuery({
    queryKey: ['launch-context'],
    queryFn: fetchLaunchContext,
    staleTime: 60 * 60 * 1000, // 1 hour - launch context rarely changes
    retry: 3,
  })

  return {
    launchContext,
    isLoading,
    isError,
    error,
  }
}

export function useClients(): ClientHookResult {
  const { launchContext } = useLaunchContext()
  const tenantId = launchContext?.tenantId

  // Create FHIR clients based on launch context
  const clients = React.useMemo(() => {
    if (!tenantId) {
      return {
        ehrClient: undefined,
        cpsClient: undefined,
        scpClient: undefined,
      }
    }

    return {
      ehrClient: createEhrClient(tenantId),
      cpsClient: createCpsClient(tenantId),
      scpClient: createScpClient(tenantId),
    }
  }, [tenantId])

  return clients
}
