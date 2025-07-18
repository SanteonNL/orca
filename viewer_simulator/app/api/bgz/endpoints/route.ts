import { NextRequest, NextResponse } from 'next/server';

export async function GET(req: NextRequest) {
    return NextResponse.json(
        Object.keys(process.env)
            .filter((key) => key.endsWith('_ORCA_URL'))
            .map((key) => ({
                name: key.slice(0, 0 - '_ORCA_URL'.length),
                endpoint: process.env[key],
            })),
    );
}
