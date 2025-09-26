import { NextRequest, NextResponse } from 'next/server';
import { DefaultAzureCredential } from '@azure/identity';

export async function POST(req: NextRequest) {
    try {
        const fhirBaseUrl = process.env.FHIR_BASE_URL;

        if (!fhirBaseUrl) {
            return NextResponse.json({ message: 'FHIR_BASE_URL is not defined' }, { status: 500 });
        }

        console.log(`Forwarding Bundle POST request to ${fhirBaseUrl}`)

        // Get authentication token for Azure FHIR if not in local environment
        let token: string | null = null;
        if (!fhirBaseUrl.includes('localhost') && !fhirBaseUrl.includes('fhirstore')) {
            try {
                const credential = new DefaultAzureCredential();
                const tokenResponse = await credential.getToken(`${fhirBaseUrl}/.default`);
                token = tokenResponse.token;
            } catch (error) {
                console.error('Azure authentication failed:', error);
                return NextResponse.json({ message: 'Azure authentication failed', error: error }, { status: 500 });
            }
        }

        const headers: HeadersInit = {
            'Content-Type': 'application/json',
        };

        if (token) {
            headers['Authorization'] = `Bearer ${token}`;
        }

        // Forward the request to the FHIR server
        const response = await fetch(fhirBaseUrl, {
            method: 'POST',
            headers: headers,
            body: await req.text(), // Pass the request body directly
        });

        if (!response.ok) {
            const errorText = await response.text();
            return NextResponse.json({ message: errorText }, { status: response.status });
        }

        const data = await response.json();
        return NextResponse.json(data, { status: 200 });
    } catch (error: any) {
        return NextResponse.json({ message: 'Internal Server Error', error: error?.message }, { status: 500 });
    }
}
