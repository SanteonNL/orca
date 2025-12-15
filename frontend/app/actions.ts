"use server"

export async function getSupportContactEmail() {
    return process.env.SUPPORT_CONTACT_EMAIL ?? null
}

export async function getPatientViewerTestUrl() {
    return process.env.PATIENT_VIEWER_URL
}
