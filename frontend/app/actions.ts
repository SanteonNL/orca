"use server"

export async function getSupportContactLink() {
    return process.env.SUPPORT_CONTACT_LINK
}

export async function getPatientViewerTestUrl() {
    return process.env.PATIENT_VIEWER_URL
}
