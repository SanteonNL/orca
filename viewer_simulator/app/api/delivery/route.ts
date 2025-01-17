import { NextRequest, NextResponse } from 'next/server';
import {Bundle} from "fhir/r4";
import { storeNotificationBundle } from './storage';

export async function POST(req: NextRequest) {
    try {
        const bundle = await req.json() as Bundle;
        storeNotificationBundle(bundle);

        return NextResponse.json({message: 'Notification bundle received successfully'}, {status: 200});
    } catch (error) {
        const errorMessage = error instanceof Error ? error.message : 'Unknown error';
        return NextResponse.json({ error: errorMessage }, { status: 500 });

    }
}