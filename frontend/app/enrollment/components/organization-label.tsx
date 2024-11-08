import { Reference } from 'fhir/r4';

import React from 'react'

interface Props {
    reference?: Reference
}

// OrganizationLabel renders an organization label with the organization's display name and identifier.
export default function OrganizationLabel({ reference }: Props) {

    if (!reference) {
        return <span>No Organization Reference found</span>
    }

    const displayName = reference.display;

    // If the identifier has no system or value, simply return the displayName, or "unknown" if no displayName is present.
    if (!reference.identifier || !reference.identifier.system || !reference.identifier.value) {
        return <span>{displayName || "unknown"}</span>
    }

    const isUraIdentifier = reference.identifier.system === 'http://fhir.nl/fhir/NamingSystem/ura'
    const identifierValue = isUraIdentifier ?
        `URA ${reference.identifier.value}` : `${reference.identifier.system}: ${reference.identifier.value}`;

    return <span>{displayName ? `${displayName} (${identifierValue})` : identifierValue}</span>
}