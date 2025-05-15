import {Identifier} from "fhir/r4"

export function getOwnIdentifier(name: string): Identifier {
    const ownIdentifier = `${process.env[`${name}_IDENTIFIER`]}`
    if (!ownIdentifier) {
        throw new Error(`Own identifier not configured ('${name}_IDENTIFIER', in the format of system|value)`);
    }
    const [system, value] = ownIdentifier.split('|');
    if (!system || !value) {
        throw new Error(`Own identifier is invalid ('${name}_IDENTIFIER', in the format of system|value)`);
    }
    return {
        system: system,
        value: value,
    }
}

export function getORCABaseURL(name: string): string {
    const orcaBaseURL = `${process.env[`${name}_ORCA_URL`]}`;
    if (!orcaBaseURL) {
        throw new Error(`ORCA base URL not configured ('${name}_ORCA_URL')`);
    }
    return orcaBaseURL;
}

export function getORCABearerToken(name: string): string {
    const bearerToken = `${process.env[`${name}_ORCA_BEARERTOKEN`]}`
    if (!bearerToken) {
        throw new Error(`Bearer token to call own ORCA not configured ('${name}_ORCA_BEARERTOKEN')`);
    }
    return bearerToken;
}