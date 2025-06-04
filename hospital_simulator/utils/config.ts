"use server"

import {Bundle, Identifier, Organization} from "fhir/r4"

export const getEnrollmentUrl = async (patientId: string, serviceRequestId: string) => {

    const practitioners = await fetch(`${process.env.FHIR_BASE_URL}/Practitioner`)
    if (!practitioners.ok) {
        throw new Error(`Failed to fetch ${process.env.FHIR_BASE_URL}/Practitioner: ${practitioners.statusText}`)
    }

    const respBundle = await practitioners.json() as Bundle

    if (respBundle && respBundle.entry?.length) {

        return `${process.env.ORCA_BASE_URL}/demo-app-launch?` + new URLSearchParams({
            patient: patientId,
            serviceRequest: `ServiceRequest/${serviceRequestId}`,
            practitioner: `Practitioner/${respBundle.entry[0].resource?.id}`, //TODO: Rework to get reference from ServiceRequest.requester - currently an Organization, but should be a PractitionerRole
            iss: `${process.env.FHIR_BASE_URL}`,
            taskIdentifier: `http://demo-launch/fhir/NamingSystem/task-identifier|${serviceRequestId}`
        }).toString()
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
