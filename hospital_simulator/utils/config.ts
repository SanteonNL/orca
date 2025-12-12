"use server"

import {Bundle, Organization} from "fhir/r4"
import { addFhirAuthHeaders } from '@/utils/azure-auth';

export const getEnrollmentUrl = async (patientId: string, serviceRequestId: string | undefined) => {
    if (!process.env.TENANT_ID) {
        throw new Error('TENANT_ID is not defined');
    }

    const headers = await addFhirAuthHeaders();

    const practitioners = await fetch(`${process.env.FHIR_BASE_URL}/Practitioner`, {
        headers: headers
    })
    if (!practitioners.ok) {
        throw new Error(`Failed to fetch ${process.env.FHIR_BASE_URL}/Practitioner: ${practitioners.statusText}`)
    }

    const respBundle = await practitioners.json() as Bundle

    if (respBundle && respBundle.entry?.length) {
        const params: Record<string, string> = {
            patient: patientId,
            practitioner: `Practitioner/${respBundle.entry[0].resource?.id}`,
            tenant: `${process.env.TENANT_ID}`
        };
        if (serviceRequestId) {
            params.serviceRequest = `ServiceRequest/${serviceRequestId}`;
            params.taskIdentifier = `http://demo-launch/fhir/NamingSystem/task-identifier|${serviceRequestId}`
        }
        return `${process.env.ORCA_BASE_URL}/demo-app-launch?` + new URLSearchParams(params).toString()
    }

}

export const getLocalOrganization = async () => {
    const ura = process.env.ORCA_LOCAL_ORGANIZATION_URA;
    if (!ura) {
        throw new Error('ORCA_LOCAL_ORGANIZATION_URA is not defined');
    }
    const name = process.env.ORCA_LOCAL_ORGANIZATION_NAME;
    if (!name) {
        throw new Error('ORCA_LOCAL_ORGANIZATION_NAME is not defined');
    }
    return toOrganization(name, ura);
}

export const getTaskPerformerOrganization = async () => {
    const ura = process.env.ORCA_PERFORMER_ORGANIZATION_URA;
    if (!ura) {
        throw new Error('ORCA_PERFORMER_ORGANIZATION_URA is not defined');
    }
    const name = process.env.ORCA_PERFORMER_ORGANIZATION_NAME;
    if (!name) {
        throw new Error('ORCA_PERFORMER_ORGANIZATION_NAME is not defined');
    }
    return toOrganization(name, ura);
}

function toOrganization(name: string, ura: string) {
    return {
        resourceType: "Organization",
        name: name,
        identifier: [
            {
                system: "http://fhir.nl/fhir/NamingSystem/ura",
                value: ura
            }
        ],
    } as Organization;
}
