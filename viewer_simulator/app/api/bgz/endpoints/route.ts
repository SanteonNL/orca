import { NextRequest, NextResponse } from 'next/server';

export async function GET(req: NextRequest) {
    return NextResponse.json(
        Object.keys(process.env)
            .filter((key) => key.endsWith('CAREPLANSERVICE_URL'))
            .map((key) => ({
                name: key.slice(0, 0 - '_CAREPLANSERVICE_URL'.length),
                endpoint: process.env[key],
            })),
    );
}
