import {Condition, ServiceRequest, Task} from "fhir/r4";
import {codingLabel, selectMappedCoding} from "@/app/utils/mapping";

// requestTitle creates a title for an order, which can be shown to the user.
export function requestTitle(serviceRequest: ServiceRequest | undefined): string | undefined {
    if (!serviceRequest?.code) {
        return undefined;
    }
    const coding = selectMappedCoding(serviceRequest.code.coding ?? []);
    if (coding) {
        return codingLabel(coding)
    }
    return undefined
}

// conditionTitle creates a title for a condition, which can be shown to the user.
// It takes the order Task and Condition, and determines a suitable title, given precedence to:
// - Task.reasonCode
// - Condition.code
// It then returns the mapped label for the coding, if available.
// If there's no mapped label, it returns a concatenation of the coding's display or code values.
export function conditionTitle(task: Task | undefined, condition: Condition | undefined): string | undefined {
    const codings = task?.reasonCode?.coding ?? [];
    if (condition?.code?.coding) {
        codings.push(...condition.code.coding);
    }
    const coding = selectMappedCoding(codings);
    if (coding) {
        const label = codingLabel(coding)
        if (label) {
            return label;
        }
    }
    const displays = codings.map(c => c.display ?? c.code).filter(c => c !== undefined);
    if (displays.length > 0) {
        return displays.join(", ");
    }
    return undefined;
}