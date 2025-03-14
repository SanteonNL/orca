"use server"

import { Bundle } from "fhir/r4"

export async function getSupportContactLink() {
    return process.env.SUPPORT_CONTACT_LINK
}

export async function viewerFeatureIsEnabled() {
    return process.env.VIEWER_ENABLED === "true"
}

export async function getViewerData(scpContext: string) {

    console.log("Getting viewer data for X-Scp-Context: ", scpContext);

    const requestUrl = `${process.env.FHIR_AGGREGATE_URL}/Observation/_search`;
    console.log(`Sending request to ${requestUrl} with X-Scp-Context: ${scpContext}`);

    const response = await fetch(requestUrl, {
        method: 'POST',
        headers: {
            "Authorization": `Bearer ${process.env.FHIR_AUTHORIZATION_TOKEN}`,
            "X-Scp-Context": scpContext,
            "Content-Type": "application/x-www-form-urlencoded",
        },
    });

    if (!response.ok) {
        console.error(`Failed to fetch ${requestUrl}: ${response.statusText}`);
        return { error: true, message: response.statusText };
    }

    const result = await response.json() as Bundle
    console.log(`Received ${result.total} Observation entries`);

    return {
        error: false,
        message: "Success",
        data: result.entry
            ?.filter((entry) => entry.resource?.resourceType === "Observation")
            ?.map((entry) => entry.resource)
    };
}