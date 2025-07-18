import { NextRequest, NextResponse } from 'next/server';

type Params = Promise<{ id: string }>

export async function PATCH(req: NextRequest, { params }: { params: Params }) {
    try {
        const fhirBaseUrl = process.env.FHIR_BASE_URL;

        if (!fhirBaseUrl) {
            return NextResponse.json({ message: 'FHIR_BASE_URL is not defined' }, { status: 500 });
        }

        const patchData = await req.json(); // Extract the patch data from the request body
        const { id } = await params; // Extract the ServiceRequest ID from the URL

        const patchResponse = await fetch(`${fhirBaseUrl}/ServiceRequest/${id}`, {
            method: 'PATCH',
            headers: { 'Content-Type': 'application/json-patch+json' },
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
