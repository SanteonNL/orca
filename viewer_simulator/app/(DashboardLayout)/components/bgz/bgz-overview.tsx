import React from 'react';

import useFetch from '@/app/useFetch';
import CarePlanTable, { Row } from './bgz-careplan-table';

const buildUrl = (
    base: string,
    path: string,
    params: Record<string, string>,
) => {
    let url = `${base}${path}?`;

    for (const key in params) {
        url += `${key}=${encodeURIComponent(params[key])}&`;
    }

    return url;
};

export default function BgzOverview({
    name,
    roles: roles = '',
}: {
    name: string;
    roles: string;
}) {
    const { loading, data: rows } = useFetch<Row[]>(
        buildUrl(
            process.env.NEXT_PUBLIC_BASE_PATH || '',
            '/api/bgz/careplans',
            {
                name,
                role: roles,
            },
        ),
    );

    return (
        <div>
            <CarePlanTable name={name} rows={rows || []} loading={loading} />
        </div>
    );
}
