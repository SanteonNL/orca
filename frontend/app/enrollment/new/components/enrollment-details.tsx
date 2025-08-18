"use client"
import React from 'react'
import useEnrollment from '@/app/hooks/enrollment-hook'
import {Spinner} from '@/components/spinner'
import {patientName, organizationName, organizationNameShort} from "@/lib/fhirRender";

export default function EnrollmentDetails() {

    const {serviceRequest, taskCondition, patient, isLoading} = useEnrollment()

    if (isLoading) return <Spinner className="h-12 w-12 text-primary"/>

    const serviceRequestDisplay = serviceRequest?.code?.coding?.[0]?.display
    const taskPerformer = serviceRequest?.performer?.[0]

    let topText = "Je gaat deze patient aanmelden."
    if (serviceRequestDisplay && taskPerformer) {
        topText = `Je gaat deze patient aanmelden voor ${serviceRequestDisplay.toLowerCase()} van ${organizationNameShort(taskPerformer)}.`
    }

    return <>
        <div className="text-muted-foreground">
            {topText}
        </div>
        <h2 className="text-xl font-bold mb-4">Patiëntgegevens</h2>
        <div className="grid grid-cols-[1fr_2fr] gap-y-4 w-[568px]">
            <div className="font-medium">Patiënt:</div>
            <div>{patient ? patientName(patient) : "Onbekend"}</div>

            <div className="font-medium">E-mailadres:</div>
            <div>{patient?.telecom?.find(m => m.system === 'email')?.value ?? 'Onbekend'}</div>

            <div className="font-medium">Telefoonnummer:</div>
            <div>{patient?.telecom?.find(m => m.system === 'phone')?.value ?? 'Onbekend'}</div>

            <div className="font-medium">Verzoek:</div>
            <div className="first-letter:uppercase">{serviceRequest?.code?.coding?.[0]?.display ?? "Onbekend"}</div>

            <div className="font-medium">Diagnose:</div>
            <div className="first-letter:uppercase">
                {taskCondition?.code?.text || taskCondition?.code?.coding?.[0].display || "Onbekend"}
            </div>

            <div className="font-medium">Uitvoerende organisatie:</div>
            <div>
                {organizationName(serviceRequest?.performer?.[0])}
            </div>
        </div>
    </>
}