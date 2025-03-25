import { CarePlan, Task } from "fhir/r4"
import { FHIR_SCP_CONTEXT_SYSTEM } from "./const/const";

export function getBsn(carePlan?: CarePlan) {
    const identifier = carePlan?.subject?.identifier;
    if (identifier?.system === 'http://fhir.nl/fhir/NamingSystem/bsn') {
        return identifier.value || "Unknown";
    }
    return "Unknown";
}

//This function takes in a Task and searches for a Task.basedOn CarePlan reference OR identifier. 
//This can be used to, e.g. map a Task to its SCP context.
//If the task contains a relative reference, the taskFullUrl is used to construct an absolute reference.
export function getScpContext(task: Task, taskFullUrl?: string) {

    const basedOnRefs = task.basedOn?.filter(basedOn => basedOn.reference?.includes("CarePlan/"))
    if (!basedOnRefs || basedOnRefs.length === 0) return

    //TODO: The code assumes a Task is bound to one SCP context. Could this, in theory, be mapped to multiple SCPs
    if (basedOnRefs.length > 1) console.warn("Task has multiple CarePlan basedOn references. Only the first one will be used.")

    //Check if any reference simply points to the SCP context via its identifier property
    const scpContextRefWithScpIdentifier = basedOnRefs.find(basedOn => basedOn.identifier && basedOn.identifier.system === FHIR_SCP_CONTEXT_SYSTEM)
    if (scpContextRefWithScpIdentifier) {
        return scpContextRefWithScpIdentifier.identifier
    }

    const refToUse = basedOnRefs[0].reference

    // Check if the the reference is already an absolute reference
    if (refToUse?.includes("http")) {
        return {
            system: FHIR_SCP_CONTEXT_SYSTEM,
            value: basedOnRefs[0].reference
        }
    }

    //otherwise, the reference MUST be relative
    if (!refToUse?.startsWith("CarePlan/")) {
        console.warn("Task has a basedOn reference that is not a CarePlan reference. Unable to determine SCP context.")
        throw new Error("Task has a basedOn reference that is not a CarePlan reference. Unable to determine SCP context.")
    }

    //TODO: There is a mismatch between the fullUrl sent as a notification, and the expected SCP context provided by the CPC
    const baseUrl = taskFullUrl?.split("Task/")[0].replace("/fhir", "/orca/cps")

    return {
        "system": FHIR_SCP_CONTEXT_SYSTEM,
        "value": baseUrl + refToUse
    }
}