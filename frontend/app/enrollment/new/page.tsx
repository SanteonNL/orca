"use client"

import React from 'react'
import {Card, CardContent, CardHeader} from '@/components/ui/card'
import EnrollInCpsButton from './components/enroll-in-cps-button'
import EnrollmentDetails from './components/enrollment-details'
import TaskHeading from "@/app/enrollment/components/task-heading";
import TaskBody from "@/app/enrollment/components/task-body";
import useEnrollment from "@/app/hooks/enrollment-hook";
import {ChevronRight} from "lucide-react";

export default function ConfirmDataPreEnrollment() {
    const {serviceRequest} = useEnrollment()
    const serviceRequestDisplay = serviceRequest?.code?.coding?.[0].display;
    return (
        <>
            <TaskHeading title={"Controleer patiëntgegevens"}>
                <nav className="flex items-center space-x-2 text-sm">
                    <span>Controleer patiëntgegevens</span>
                    {serviceRequestDisplay ?
                        <>
                            <ChevronRight className="h-4 w-4"/>
                            <span className={`first-letter:uppercase text-muted-foreground`}>{serviceRequestDisplay} instellen</span>
                        </>
                        : <></>
                    }
                </nav>
            </TaskHeading>
            <TaskBody>
                <Card className="border-0 shadow-none px-0">
                    <CardContent className="space-y-6 px-0">
                        <EnrollmentDetails/>
                    </CardContent>
                    <CardHeader className="px-0 space-y-0 pt-0 pb-8">
                        <p className="text-muted-foreground max-w-[560px]">
                            Indien de gegevens van de patiënt niet kloppen, pas het dan aan in het EPD. Sluit daarna dit scherm en open het opnieuw om de wijzigingen te zien.
                        </p>
                    </CardHeader>
                    <EnrollInCpsButton className='mt-5'/>
                </Card>
            </TaskBody>
        </>
    )
}

