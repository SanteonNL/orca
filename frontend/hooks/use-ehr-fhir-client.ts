'use client'
import { useMemo } from 'react';
import Client from 'fhir-kit-client';

const useEhrFhirClient = () => {
    const client = useMemo(() => {
        if (typeof window !== 'undefined') {
            return new Client({ baseUrl: `${window.location.origin}/orca/contrib/ehr/fhir` });
        }
        return null;
    }, []);

    return client;
};

export default useEhrFhirClient;
