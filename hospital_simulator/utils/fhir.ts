import {HumanName, Identifier, Patient} from "fhir/r4";

export function TokenToIdentifier(str: string): Identifier | undefined {
    if (!str) {
        return undefined;
    }
    const parts = str.split('|');
    if (parts.length < 2) {
        console.error(`Invalid identifier format: ${str}`);
        return undefined;
    }
    return {
        system: parts[0],
        value: parts[1]
    };
}

export const FormatHumanName = (name: HumanName | undefined): string => {
    if (!name) {
        return "(no name)";
    }
    if (name?.text) {
        return name.text;
    }
    if (!name.family && name.given?.length == 0) {
        return "(empty name)";
    }
    // return as: <family name>, <given names (space separated)>
    return [name.family, name.given?.join(" ")].filter(Boolean).join(", ");
}