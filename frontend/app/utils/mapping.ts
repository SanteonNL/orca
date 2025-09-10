import {Reference} from "fhir/r4";
import {organizationNameShort} from "@/lib/fhirRender";

export function statusLabelLong(taskStatus: string, serviceRequestDisplay?: string, taskOwner?: Reference): string {
    if (!serviceRequestDisplay || !taskOwner) {
        return taskStatusLabel(taskStatus);
    }
    const serviceRequestDisplayCased = titleCase(serviceRequestDisplay)
    switch (taskStatus) {
        case "ready":
            return `${serviceRequestDisplayCased} instellen`;
        case "requested":
            return `${serviceRequestDisplayCased} instellen`;
        case "received":
            return `${serviceRequestDisplayCased} instellen`;
        case "accepted":
            return `Aanmelding voor ${serviceRequestDisplay.toLowerCase()} ${organizationNameShort(taskOwner)} is gelukt!`;
        case "in-progress":
            return `${serviceRequestDisplayCased} beschikbaar`;
        default:
            return taskStatusLabel(taskStatus);
    }
}

export function taskStatusLabel(taskStatus: string): string {
    switch (taskStatus) {
        case "ready":
            return "Instellen"
        case "accepted":
            return "Aanmelding gelukt"
        case "completed":
            return "Afgerond"
        case "cancelled":
            return "Geannuleerd"
        case "failed":
            return "Mislukt"
        case "in-progress":
            return "In behandeling"
        case "on-hold":
            return "Gepauzeerd"
        case "requested":
            return "Instellen"
        case "received":
            return "Instellen"
        case "rejected":
            return "Aanmelding afgewezen"
        default:
            return taskStatus
    }
}

const codingLabels = {
    "http://snomed.info/sct|719858009": "thuismonitoring",
    "http://snomed.info/sct|84114007": "hartfalen",
    "http://snomed.info/sct|13645005": "COPD",
    "http://snomed.info/sct|195967001": "astma",
}

type coding = {
    system?: string; code?: string; display?: string
}

// firstKnownCoding returns the first coding in the list that has a known label.
// If there's no known coding, it returns undefined.
export function selectMappedCoding(codings: coding[]): coding | undefined {
    for (const coding of codings) {
        if (coding.system && coding.code) {
            const key = `${coding.system}|${coding.code}`;
            if (key in codingLabels) {
                return coding;
            }
        }
    }
    return undefined;
}

export function codingLabel(coding: coding): string | undefined {
    if (coding.system && coding.code) {
        const key = `${coding.system}|${coding.code}`;
        if (key in codingLabels) {
            return codingLabels[key as keyof typeof codingLabels];
        }
    }
    return coding.display
}

export function titleCase(str: string): string {
    return str.charAt(0).toUpperCase() + str.slice(1).toLowerCase();
}