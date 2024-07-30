'use client'
import { useMemo } from 'react';
import Client from 'fhir-kit-client';

const useCpsClient = () => {
    const client = useMemo(() => {
        if (typeof window !== 'undefined') {
            return new Client({ baseUrl: `${window.location.origin}/orca/contrib/cps/fhir` });
            // return new Client({ baseUrl: `http://localhost:7090/fhir` });
        }
        return null;
    }, []);

    return client;
};

export default useCpsClient;
