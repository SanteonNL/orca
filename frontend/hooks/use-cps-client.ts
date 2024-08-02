'use client'
import { useMemo } from 'react';
import { createCpsClient } from '@/lib/fhirUtils';

const useCpsClient = () => {
    const client = useMemo(() => {
        if (typeof window !== 'undefined') {
            return createCpsClient();
        }
        return null;
    }, []);

    return client;
};

export default useCpsClient;
