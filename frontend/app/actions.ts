"use server"

import { Bundle } from "fhir/r4"

export async function getSupportContactLink() {
    return process.env.SUPPORT_CONTACT_LINK
}

export async function viewerFeatureIsEnabled() {
    return process.env.DATA_VIEWER_ENABLED === "true"
}

export async function getAggregationUrl() {
    return process.env.FHIR_AGGREGATE_URL
}

export async function getPatientViewerUrl() {
    return process.env.PATIENT_VIEWER_URL
}
