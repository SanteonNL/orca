import {Box, Typography} from '@mui/material';
import PageContainer from '@/app/(DashboardLayout)/components/container/PageContainer';
import DashboardCard from '@/app/(DashboardLayout)/components/shared/DashboardCard';
import Overview from '@/app/(DashboardLayout)/components/service-requests/overview';
import {FormatHumanName, ReadPatient, TokenToIdentifier} from "@/utils/fhir";
import React from "react";

export default async function ServiceRequestsPage(
    {params, searchParams}: {
        params: Promise<{ slug: string }>
        searchParams: Promise<{ [key: string]: string | string[] | undefined }>
    }) {
    const patientIdentifierStr = (await searchParams).patient as string | undefined;
    if (!patientIdentifierStr) {
        return <>missing patient identifier</>;
    }
    const patientIdentifier = TokenToIdentifier(patientIdentifierStr);
    if (!patientIdentifier) {
        return <>invalid patient identifier: {patientIdentifierStr}</>;
    }
    const patient = await ReadPatient(patientIdentifier);
    if (!patient) {
        return <>patient not found: {patientIdentifierStr}</>;
    }
    return (
        <Box sx={{position: 'relative'}}>
            <PageContainer
                title="Service Requests"
                description={"Shows all service requests, create new requests and view its status for patient: " + FormatHumanName(patient.name!![0]) }
            >
                <DashboardCard title="Service Requests">
                    <>
                        <Typography>Service requests for patient: {FormatHumanName(patient.name!![0])}</Typography>
                        <Overview patientID={patient.id!!}/>
                    </>
                </DashboardCard>
            </PageContainer>
        </Box>

    );
};
