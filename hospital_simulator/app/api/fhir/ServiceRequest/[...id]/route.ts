import { NextRequest, NextResponse } from 'next/server';
import { DefaultAzureCredential } from '@azure/identity';

type Params = Promise<{ id: string[] }>

export async function PATCH(req: NextRequest, { params }: { params: Params }) {
    try {
        const fhirBaseUrl = process.env.FHIR_BASE_URL;

        if (!fhirBaseUrl) {
            return NextResponse.json({ message: 'FHIR_BASE_URL is not defined' }, { status: 500 });
        }

        const patchData = await req.json(); // Extract the patch data from the request body
        const { id: idArray } = await params; // Extract the ServiceRequest ID array from the URL
        const id = idArray[0]; // Get the first element since we expect a single ID

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
            'Content-Type': 'application/json-patch+json'
        };

        if (token) {
            headers['Authorization'] = `Bearer ${token}`;
        }

        const patchResponse = await fetch(`${fhirBaseUrl}/ServiceRequest/${id}`, {
            method: 'PATCH',
            headers: headers,
            body: JSON.stringify(patchData), // Send the patch data
        });

        if (!patchResponse.ok) {
            const errorText = await patchResponse.text();
            return NextResponse.json({ message: errorText }, { status: patchResponse.status });
        }

        const patchResult = await patchResponse.json();
        return NextResponse.json(patchResult, { status: 200 });
    } catch (error: any) {
        return NextResponse.json({ message: 'Internal Server Error', error: error?.message }, { status: 500 });
    }
}
