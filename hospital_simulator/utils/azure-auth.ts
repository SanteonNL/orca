import {DefaultAzureCredential} from '@azure/identity';

/**
 * Gets headers for FHIR API requests, including Azure authentication if needed.
 * Automatically skips authentication for local development environments (localhost/fhirstore).
 *
 * @param baseHeaders - Base headers to include
 * @returns Headers with authentication added if needed
 * @throws Error if authentication fails in non-local environments
 */
export async function addFhirAuthHeaders(
    baseHeaders: HeadersInit = {}
): Promise<HeadersInit> {
    const url = process.env.FHIR_BASE_URL || '';
    const headers: Record<string, string> = {...(baseHeaders as Record<string, string>)};

    if (url.includes('localhost') || url.includes('fhirstore')) {
        return headers;
    }

    try {
        const credential = new DefaultAzureCredential();
        const tokenResponse = await credential.getToken(`${url}/.default`);
        headers['Authorization'] = `Bearer ${tokenResponse.token}`;
    } catch (error) {
        console.error('Azure authentication failed:', error);
        throw error;
    }

    return headers;
}