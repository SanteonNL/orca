import { NextRequest, NextResponse } from 'next/server';

export async function POST(req: NextRequest) {
    try {
        const fhirBaseUrl = process.env.FHIR_BASE_URL;

        if (!fhirBaseUrl) {
            return NextResponse.json({ message: 'FHIR_BASE_URL is not defined' }, { status: 500 });
        }

        console.log(`Forwarding Bundle POST request to ${fhirBaseUrl}`)

        // Forward the request to the FHIR server
        const response = await fetch(fhirBaseUrl, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
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
