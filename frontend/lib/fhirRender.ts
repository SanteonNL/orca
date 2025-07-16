import {Patient, Reference} from 'fhir/r4';

export const patientName = (patient: Patient): string => {
    if (!patient.name || patient.name.length == 0) {
        return "(no name)";
    }
    const name = patient.name?.[0];
    if (name?.text) {
        return name.text;
    }
    if (!name.family && name.given?.length == 0) {
        return "(empty name)";
    }
    // return as: <family name>, <given names (space separated)>
    return [name.family, name.given?.join(" ")].filter(Boolean).join(", ");
}

export const findTelecom = (patient: Patient): Telecom => {
    let phone = "Onbekend";
    let email = "Onbekend";

    if (patient.telecom) {
        for (const telecom of patient.telecom) {
            if (telecom.system === 'phone') {
                phone = telecom.value || "Onbekend";
            } else if (telecom.system === 'email') {
                email = telecom.value || "Onbekend";
            }
        }
    }
    return new Telecom(phone, email);
}


export class Telecom {
    constructor(
        public readonly telephone: string,
        public readonly email: string
    ) {
    }
}


export const organizationName = (reference?: Reference) => {
    if (!reference) {
        return "No Organization Reference found"
    }
    const displayName = reference.display;

    // If the identifier has no system or value, simply return the displayName, or "unknown" if no displayName is present.
    if (!reference.identifier || !reference.identifier.system || !reference.identifier.value) {
        return displayName || "unknown"
    }

    const isUraIdentifier = reference.identifier.system === 'http://fhir.nl/fhir/NamingSystem/ura'
    const identifierValue = isUraIdentifier ?
        `URA ${reference.identifier.value}` : `${reference.identifier.system}: ${reference.identifier.value}`;

    return displayName ? `${displayName} (${identifierValue})` : identifierValue
}