import {Reference} from 'fhir/r4';

// OrganizationLabel renders an organization label with the organization's display name and identifier.
export default function OrganizationLabel(reference: Reference) {
    let identifierValue =
        reference.identifier?.system === 'http://fhir.nl/fhir/NamingSystem/ura'
            ? 'URA' : reference.identifier?.system + ': ' + reference?.identifier?.value
    const displayName = reference?.organization?.display;
    return <span>{displayName ? displayName + ' (' + identifierValue + ')' : identifierValue}</span>
}