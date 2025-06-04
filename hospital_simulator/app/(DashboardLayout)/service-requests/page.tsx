import {Box, Typography} from '@mui/material';
import PageContainer from '@/app/(DashboardLayout)/components/container/PageContainer';
import DashboardCard from '@/app/(DashboardLayout)/components/shared/DashboardCard';
import Page from '@/app/(DashboardLayout)/components/service-requests/page';
import {FormatHumanName} from "@/utils/fhir";
import {Bundle, Patient} from "fhir/r4";
import React from "react";

export default async function ServiceRequestsPage(
    {params, searchParams}: {
        params: Promise<{ slug: string }>
        searchParams: Promise<{ [key: string]: string | string[] | undefined }>
    }) {
    const patientIdentifier = (await searchParams).patient as string | undefined;
    if (!patientIdentifier) {
        return <>missing patient identifier</>;
    }
    const patientResponse = await fetch(`${process.env.FHIR_BASE_URL}/Patient?identifier=${encodeURIComponent(patientIdentifier)}`, {
        cache: 'no-store',
        headers: {
            "Cache-Control": "no-cache"
        }
    });
    if (!patientResponse.ok) {
        const errorText = await patientResponse.text();
        console.error('Failed to fetch patient: ', errorText);
        throw new Error('Failed to fetch patient: ' + errorText);
    }
    const patientBundle = await patientResponse.json() as Bundle<Patient>;
    if (patientBundle.entry?.length == 0) {
        return <>No patient found for identifier: {patientIdentifier}</>;
    }
    const patient = patientBundle.entry!![0]!!.resource!! as Patient;

    return (
        <Box sx={{position: 'relative'}}>
            <PageContainer
                title="Service Requests"
                description={"Shows all service requests, create new requests and view its status for patient: " + FormatHumanName(patient.name!![0]) }
            >
                <DashboardCard title="Service Requests">
                    <>
                        <Typography>Service requests for patient: {FormatHumanName(patient.name!![0])}</Typography>
                        <Page patient={patient}/>
                    </>
                </DashboardCard>
            </PageContainer>
        </Box>

    );
};
