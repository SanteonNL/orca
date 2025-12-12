import {Patient, Reference} from 'fhir/r4';

export const patientName = (patient: Patient): string => {
    // Name will be returned as <First Name> <Family Name>, if these are not present, fallback to name.text
    // Even if initials are present, they will not be displayed

    if (!patient.name || patient.name.length == 0) {
        return "(no name)";
    }

    const name = patient.name?.[0];

    if (name?.given && name.given.length > 0 && name.family) {
        return `${name.given[0]} ${name.family}`;
    }
    if (name?.text) {
        return name.text;
    }
    return "(empty name)";
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

// organizationNameShort is a shorter version of organizationName that omits the display name, or the reference value if no display name is present.
export const organizationNameShort = (reference: Reference) => {
    if (reference.display) {
        return reference.display;
    }
    if (!reference.identifier?.value) {
        return "(unknown)";
    }

    return `(${reference.identifier.system}: ${reference.identifier.value})`
}