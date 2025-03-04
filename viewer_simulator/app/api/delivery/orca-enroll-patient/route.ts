import { NextRequest, NextResponse } from 'next/server';
import {Bundle} from "fhir/r4";
import { storeNotificationBundle } from './storage';

export async function POST(req: NextRequest) {
    try {
        console.log('Received notification bundle');

        // The bundles are in the object under the key 'bundles'
        // @ts-ignore
        const { bundles } = await req.json() as Bundle[];

        console.log('Bundle: ', bundles);
        for (const bundle of bundles) {
            storeNotificationBundle(bundle);
        }

        return NextResponse.json({message: 'Notification bundle received successfully'}, {status: 200});
    } catch (error) {
        const errorMessage = error instanceof Error ? error.message : 'Unknown error';
        return NextResponse.json({ error: errorMessage }, { status: 500 });

    }
}