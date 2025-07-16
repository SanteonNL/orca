"use client"
import React, {useEffect, useState} from 'react'
import useEnrollmentStore from '@/lib/store/enrollment-store'
import { Spinner } from '@/components/spinner'
import {patientName, organizationName, Telecom, findTelecom} from "@/lib/fhirRender";

export default function EnrollmentDetails() {

    const { serviceRequest, taskCondition, patient, loading } = useEnrollmentStore()
    const [telecom, setTelecom] = useState<Telecom>(new Telecom("Onbekend", "Onbekend"));

    useEffect(() => {
        if (!patient) {
            return
        }
        const telecom = findTelecom(patient)
        setTelecom(telecom);
    }, [patient]);

    if (loading) return <Spinner className="h-12 w-12 text-primary" />

    return (
        (
            <div className="grid grid-cols-[1fr_2fr] gap-y-4 w-[568px]">
                <div className="font-[500]">Patiënt:</div>
                <div>{patient ? patientName(patient) : "Onbekend"}</div>

                <div className="font-[500]">E-mailadres:</div>
                <div>{telecom.email}</div>

                <div className="font-[500]">Telefoonnummer:</div>
                <div>{telecom.telephone}</div>

                <div className="font-[500]">Verzoek:</div>
                <div className="first-letter:uppercase">{serviceRequest?.code?.coding?.[0].display || "Onbekend"}</div>

                <div className="font-[500]">Diagnose:</div>
                <div className="first-letter:uppercase">
                    {taskCondition?.code?.text || taskCondition?.code?.coding?.[0].display || "Onbekend"}
                </div>

                <div className="font-[500]">Uitvoerende organisatie:</div>
                <div>
                    {organizationName(serviceRequest?.performer?.[0])}
                </div>
            </div>
        )
    )
}