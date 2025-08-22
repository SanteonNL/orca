import {Reference} from "fhir/r4";
import {organizationNameShort} from "@/lib/fhirRender";

export function statusLabelLong(taskStatus: string, serviceRequestDisplay?: string, taskOwner?: Reference): string {
    if (!serviceRequestDisplay || !taskOwner) {
        return taskStatusLabel(taskStatus);
    }
    const serviceRequestDisplayCased = serviceRequestDisplay.slice(0, 1).toUpperCase() + serviceRequestDisplay.slice(1)
    switch (taskStatus) {
        case "ready":
            return `${serviceRequestDisplayCased} instellen`;
        case "requested":
            return `${serviceRequestDisplayCased} instellen`;
        case "received":
            return `${serviceRequestDisplayCased} instellen`;
        case "accepted":
            return "Aanmelding voor " + serviceRequestDisplay.toLowerCase() + " " + organizationNameShort(taskOwner) + " is gelukt!"
        case "in-progress":
            return `${serviceRequestDisplay[0].toUpperCase()}${serviceRequestDisplay.slice(1)} beschikbaar`;
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