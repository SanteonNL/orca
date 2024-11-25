import { NextRequest, NextResponse } from 'next/server';

//Proxies all GET requests to the configured FHIR_BASE_URL
export async function GET(req: NextRequest, { params }: { params: { fhirPath: string } }) {

    try {
        const { fhirPath } = params;
        const fhirPathUrlSegment = Array.isArray(fhirPath) ? fhirPath.join('/') : fhirPath;

        let requestHeaders = new Headers(req.headers)
        if (process.env.FHIR_AUTHORIZATION_TOKEN) {
            requestHeaders.set('Authorization', "Bearer " + process.env.FHIR_AUTHORIZATION_TOKEN);
        }
        const response = await fetch(`${process.env.FHIR_BASE_URL}/${fhirPathUrlSegment}`, {
            method: req.method,
            headers: requestHeaders,
        });

        if (!response.ok) {
            throw new Error("Failed to proxy FHIR request: " + response.statusText)
        }

        return NextResponse.json(await response.json(), { status: response.status });
    } catch (error) {
        const errorMessage = error instanceof Error ? error.message : 'Unknown error';
        return NextResponse.json({ error: errorMessage }, { status: 500 });

    }
}