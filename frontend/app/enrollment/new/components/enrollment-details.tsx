"use client"
import React from 'react'
import useEnrollmentStore from '@/lib/store/enrollment-store'
import { Spinner } from '@/components/spinner'
import { patientName, organizationName } from "@/lib/fhirRender";

export default function EnrollmentDetails() {

    const { serviceRequest, taskCondition, patient, loading } = useEnrollmentStore()

    if (loading) return <Spinner className="h-12 w-12 text-primary" />

    return (
        (
            <div className="grid grid-cols-[1fr,2fr] gap-y-4 w-[568px]">
                <div>Patiënt:</div>
                <div className="font-[500] capitalize">{patient ? patientName(patient) : "Onbekend"}</div>

                <div>Verzoek:</div>
                <div className="font-[500]">{serviceRequest?.code?.coding?.[0].display || "Onbekend"}</div>

                <div>Diagnose:</div>
                <div className="font-[500] capitalize">
                    {taskCondition?.code?.text || taskCondition?.code?.coding?.[0].display || "Onbekend"}
                </div>

                <div>Uitvoerende organisatie:</div>
                <div className="font-[500] capitalize">
                    {organizationName(serviceRequest?.performer?.[0])}
                </div>
            </div>
        )
    )
}