import {Bundle, HumanName, Identifier, Patient} from "fhir/r4";
import {DefaultAzureCredential} from '@azure/identity';

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

export async function ReadPatient(i : string | Identifier) {
    // Get authentication token for Azure FHIR if not in local environment
    let token: string | null = null;
    const fhirUrl = process.env.FHIR_BASE_URL || '';
    if (!fhirUrl.includes('localhost') && !fhirUrl.includes('fhirstore')) {
        try {
            const credential = new DefaultAzureCredential();
            const tokenResponse = await credential.getToken(`${fhirUrl}/.default`);
            token = tokenResponse.token;
        } catch (error) {
            console.error('Azure authentication failed:', error);
            throw error;
        }
    }

    const headers: HeadersInit = {
        "Cache-Control": "no-cache"
    };

    if (token) {
        headers['Authorization'] = `Bearer ${token}`;
    }

    if (typeof i === "string") {
        const httpResponse = await fetch(`${process.env.FHIR_BASE_URL}/Patient/${i}`, {
            cache: 'no-store',
            headers: headers
        });
        if (!httpResponse.ok) {
            const errorText = await httpResponse.text();
            console.error('Failed to fetch patient: ', errorText);
            throw new Error('Failed to fetch patient: ' + errorText);
        }
        return await httpResponse.json() as Patient;
    }
    const identifier = i as Identifier;
    const httpResponse = await fetch(`${process.env.FHIR_BASE_URL}/Patient?identifier=${encodeURIComponent(identifier.system + "|" + identifier.value)}`, {
        cache: 'no-store',
        headers: headers
    });
    if (!httpResponse.ok) {
        const errorText = await httpResponse.text();
        console.error('Failed to fetch patient: ', errorText);
        throw new Error('Failed to fetch patient: ' + errorText);
    }
    const searchSet = await httpResponse.json() as Bundle<Patient>;
    if (searchSet.entry?.length == 0) {
        return undefined
    }
    return searchSet.entry!![0].resource as Patient;
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