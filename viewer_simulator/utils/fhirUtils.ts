import { CarePlan } from "fhir/r4"

export function getBsn(carePlan?: CarePlan) {
    const identifier = carePlan?.subject?.identifier;
    if (identifier?.system === 'http://fhir.nl/fhir/NamingSystem/bsn') {
        return identifier.value || "Unknown";
    }
    return "Unknown";
}