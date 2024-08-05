'use client'
import { useMemo } from 'react';
import { createEhrClient } from '@/lib/fhirUtils';

const useEhrFhirClient = () => {
    const client = useMemo(() => {
        if (typeof window !== 'undefined') {
            return createEhrClient()
        }
        return null;
    }, []);

    return client;
};

export default useEhrFhirClient;
