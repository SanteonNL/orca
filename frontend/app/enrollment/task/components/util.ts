// orderTitle creates a title for an orderr, which can be shown to the user.
// It takes the order Task and ServiceRequest, and determines a suitable title, given precedence to:
// - Task.focus.display
// - If not set, then: ServiceRequest.code.text
// - If not set, then: a joined string of ServiceRequest.code.coding display or code
import {ServiceRequest, Task} from "fhir/r4";

export default function orderTitle(task: Task, serviceRequest: ServiceRequest | undefined): string | undefined {
    if (task.focus && task.focus.display) {
        return task.focus.display;
    }
    if (serviceRequest) {
        if (serviceRequest.code && serviceRequest.code.text) {
            return serviceRequest.code.text;
        } else if (serviceRequest.code && serviceRequest.code.coding && serviceRequest.code.coding.length > 0) {
            return serviceRequest.code.coding.map(coding => coding.display || coding.code).join(", ");
        }
    }
    return undefined;
}