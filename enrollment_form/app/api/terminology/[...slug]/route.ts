import { NextRequest, NextResponse } from 'next/server';
import { Mutex } from 'async-mutex';

let terminologyToken = '';
let terminologyTokenExpiration = new Date(0);
const USERNAME_NTS = process.env.USERNAME_NTS || '';
const PASSWORD_NTS = process.env.PASSWORD_NTS || '';
const tokenMutex = new Mutex();

const cache = new Map<string, { body: any, expiration: Date }>();

/**
 * This function proxies to the national terminology server. This is needed as the `@aehrc/smart-forms-renderer` 
 * library does not support auth on the terminology server, but the national terminology server requires it.
 * 
 * @param req 
 * @returns 
 */
export async function GET(req: NextRequest) {
    const slug = req.nextUrl.pathname.replace("/api/terminology/", "")
    const searchParams = req.nextUrl.searchParams;
    const token = await getToken();
    const url = `https://terminologieserver.nl/fhir/${slug}?${searchParams}`;
    const headers = { Authorization: `Bearer ${token}` };

    try {
        const data = await getBodyWithCaching(url, headers, 10000);
        return NextResponse.json(data);
    } catch (error) {
        return NextResponse.json({ error: "Failed to proxy ValueSet" }, { status: 500 });
    }
}

// Helper function to get cached response
const getCachedResponse = (key: string) => {
    const cached = cache.get(key);
    if (cached && cached.expiration > new Date()) {
        return cached.body;
    }
    return null;
};

// Helper function to cache response
const cacheResponse = (key: string, body: any, expiration: Date) => {
    cache.set(key, { body, expiration });
};

// Function to get the OAuth token
const getToken = async () => {
    if (new Date() < terminologyTokenExpiration) {
        return terminologyToken;
    }

    await tokenMutex.runExclusive(async () => {
        if (new Date() < terminologyTokenExpiration) {
            return;
        }

        const response = await fetch('https://terminologieserver.nl/auth/realms/nictiz/.well-known/openid-configuration');
        const openIdConfig = await response.json();
        const tokenEndpoint = openIdConfig.token_endpoint;

        const tokenResponse = await fetch(tokenEndpoint, {
            method: 'POST',
            headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
            body: new URLSearchParams({
                grant_type: 'password',
                client_id: 'cli_client',
                username: USERNAME_NTS,
                password: PASSWORD_NTS
            }).toString()
        });

        if (!tokenResponse.ok) {
            throw new Error(`Failed to get token: ${tokenResponse.statusText}`);
        }

        const tokenData = await tokenResponse.json();
        terminologyToken = tokenData.access_token;
        terminologyTokenExpiration = new Date(Date.now() + tokenData.expires_in * 1000);
    });

    return terminologyToken;
};

// Function to get body with caching
const getBodyWithCaching = async (url: string, headers: HeadersInit, timeout: number) => {

    const cachedResponse = getCachedResponse(url);
    if (cachedResponse) {
        return cachedResponse;
    }

    const controller = new AbortController();
    const id = setTimeout(() => controller.abort(), timeout);

    try {
        const response = await fetch(url, { headers, signal: controller.signal });
        clearTimeout(id);

        if (!response.ok) {
            throw new Error(`Failed to fetch: ${response.statusText}`);
        }

        const body = await response.json();
        cacheResponse(url, body, new Date(Date.now() + 24 * 60 * 60 * 1000)); // Cache for 24 hours
        return body;
    } catch (error: any) {
        if (error.name === 'AbortError') {
            throw new Error(`Request timed out: ${url}`);
        }
        throw error;
    }
};