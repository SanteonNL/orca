"use client"
import React from 'react'
import useEnrollmentStore from '@/lib/store/enrollment-store'
import { Spinner } from '@/components/spinner'
import organizationName from '@/lib/fhirUtils'

export default function EnrollmentDetails() {

    const { serviceRequest, taskCondition, patient, loading, launchContext } = useEnrollmentStore()

    if (loading) return <Spinner className="h-12 w-12 text-primary" />


    return (
        (
            <div className="grid grid-cols-[1fr,2fr] gap-y-4">
                <div className="text-muted-foreground">Patient:</div>
                <div className="font-bold">{patient?.name?.[0].text || "Unknown"}</div>

                <div className="text-muted-foreground">Verzoek:</div>
                <div className="font-bold">{serviceRequest?.code?.coding?.[0].display || "Unknown"}</div>

                <div className="text-muted-foreground">Diagnose:</div>
                <div className="font-bold">{taskCondition?.code?.text || taskCondition?.code?.coding?.[0].display || "Unknown"}</div>

                <div className="text-muted-foreground">Uitvoerende organisatie:</div>
                <div className="font-bold">
                    {organizationName(serviceRequest?.performer?.[0])}
                </div>
            </div>
        )
    )
}