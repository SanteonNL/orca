import React from 'react';

import useFetch from '@/app/useFetch';
import CarePlanTable, { Row } from './bgz-careplan-table';

export default function BgzOverview({ name }: { name: string }) {
    const { loading, data: rows } = useFetch<Row[]>(
        `${process.env.NEXT_PUBLIC_BASE_PATH || ''}/api/bgz/careplans?name=${encodeURIComponent(name)}`,
    );

    return (
        <div>
            <CarePlanTable name={name} rows={rows || []} loading={loading} />
        </div>
    );
}
