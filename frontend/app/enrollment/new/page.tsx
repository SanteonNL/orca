import React from 'react'
import {Card, CardContent, CardHeader} from '@/components/ui/card'
import EnrollInCpsButton from './components/enroll-in-cps-button'
import EnrollmentDetails from './components/enrollment-details'
import TaskHeading from "@/app/enrollment/components/task-heading";
import TaskBody from "@/app/enrollment/components/task-body";

export default function ConfirmDataPreEnrollment() {
    return (
        <>
            <TaskHeading title={"Verzoek controleren"}>
                <nav className="flex items-center space-x-2 text-sm">
                    <span className='font-medium'>Verzoek controleren</span>
                </nav>
            </TaskHeading>
            <TaskBody>
                <Card className="border-0 shadow-none px-0">
                    <CardHeader className="px-0 space-y-0 pt-0 pb-8">
                        <p className="text-muted-foreground">
                            Indien het verzoek niet klopt, pas het dan aan in het EPD.
                        </p>
                    </CardHeader>
                    <CardContent className="space-y-6 px-0">
                        <EnrollmentDetails/>
                    </CardContent>
                    <EnrollInCpsButton className='mt-5'/>
                </Card>
            </TaskBody>
        </>
    )
}

