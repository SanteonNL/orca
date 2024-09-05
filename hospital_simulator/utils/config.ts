"use server"

export const getEnrollmentUrl = async (patientId: string, serviceRequestId: string) => {
    return `${process.env.ORCA_BASE_URL}/demo-app-launch?` + new URLSearchParams({
        patient: patientId,
        serviceRequest: `ServiceRequest/${serviceRequestId}`,
        practitioner: "Practitioner/7", //TODO: Remove hard-coded prac
        iss: `${process.env.FHIR_BASE_URL}`
    }).toString()
}