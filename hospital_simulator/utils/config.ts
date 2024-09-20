"use server"

import { Bundle } from "fhir/r4"

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
            iss: `${process.env.FHIR_BASE_URL}`
        }).toString()
    }

}