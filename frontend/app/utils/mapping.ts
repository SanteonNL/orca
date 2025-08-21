
export function taskStatusLabel(taskStatus: string): string {
    switch (taskStatus) {
        case "accepted":
            return "Geaccepteerd"
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
            return "Verstuurd"
        case "received":
            return "Ontvangen"
        case "rejected":
            return "Afgewezen"
        default:
            return taskStatus
    }
}