"use client"

import TaskBody from "@/app/enrollment/components/task-body"
import TaskHeading from "@/app/enrollment/components/task-heading"
import { useClients, useLaunchContext } from '@/app/hooks/context-hook'
import useEnrollment from "@/app/hooks/enrollment-hook"
import { defaultTheme } from "@/app/theme"
import { Spinner } from '@/components/spinner'
import { Card, CardContent, CardHeader } from '@/components/ui/card'
import { constructTaskBundle, findInBundle, getPatientIdentifier } from '@/lib/fhirUtils'
import { Button, ThemeProvider } from '@mui/material'
import { Bundle, Coding, Condition, OperationOutcome, PractitionerRole } from 'fhir/r4'
import { ArrowRight, ChevronRight } from "lucide-react"
import { useRouter } from 'next/navigation'
import { useMemo, useState } from 'react'
import { toast } from "sonner"
import EnrollmentDetails from './components/enrollment-details'
import ValidationErrors from "@/app/enrollment/new/components/validation-errors"

export default function ConfirmDataPreEnrollment() {
    const {
        patient,
        taskCondition,
        practitionerRole,
        serviceRequest,
        isLoading,
    } = useEnrollment()

    const serviceRequestDisplay = serviceRequest?.code?.coding?.[0].display;

    const { launchContext } = useLaunchContext()
    const { cpsClient } = useClients()

    const [submitted, setSubmitted] = useState(false)
    const disabled = submitted || !taskCondition || isLoading
    const [error, setError] = useState<string | null>()
    const [validationErrors, setValidationErrors] = useState<Coding[]>()

    const router = useRouter()

    const informCps = async () => {
        setSubmitted(true)
        setError(undefined)
        setValidationErrors(undefined)

        if (!taskCondition) {
            const errorMsg = "Something went wrong with CarePlan creation"
            setError(errorMsg)
            setSubmitted(false)
            throw new Error(errorMsg)
        }

        try {
            const taskBundle = await createTask(taskCondition, practitionerRole)
            const task = findInBundle('Task', taskBundle as Bundle);

            if (!task) {
                throw new Error("Something went wrong with Task creation")
            }

            router.push(`/enrollment/task/${task.id}`)
        } catch (error: any) {
            const errorMsg = error.message || "Failed to create task"
            setError(errorMsg)
            setSubmitted(false)
        }
    }

    const createTask = async (taskCondition: Condition, practitionerRole?: PractitionerRole) => {
        if (!patient || !getPatientIdentifier(patient) || !taskCondition || !serviceRequest) {
            throw new Error("Missing required items for Task creation")
        }
        if (!cpsClient || !launchContext) {
            throw new Error("Context is not initialized")
        }

        let taskBundle: Bundle & { type: "transaction"; };

        try {
            taskBundle = constructTaskBundle(serviceRequest, taskCondition, patient, practitionerRole, launchContext.taskIdentifier);
        } catch (error) {
            console.debug("Error constructing taskBundle");
            const msg = `Failed to construct Task Bundle. Error message: ${JSON.stringify(error) ?? "Not error message found"}`;
            toast.error(msg, {richColors: true});
            throw new Error(`Failed to construct Task Bundle: ${error instanceof Error ? error.message : "Unknown error"}`);
        }

        try {
            return await cpsClient.transaction({body: taskBundle});
        } catch (error: any) {
            console.debug("Error posting Bundle", taskBundle);
            console.error(error);

            // Handle 400 errors specifically
            if (error.response?.status === 400) {
                const operationOutcome: OperationOutcome = error.response?.data;
                const operationOutcomeIssue = operationOutcome.issue?.find(issue => issue.code === 'invariant');
                const validationErrors: Coding[] = operationOutcomeIssue?.details?.coding || [];
                setValidationErrors(validationErrors);
                throw new Error("Validation errors");
            }

            throw new Error(`Failed to execute Task Bundle: ${error.message || "Unknown error"}`);
        }
    }

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
                     <ThemeProvider theme={defaultTheme}>
                        <div className="mb-8">
                            {validationErrors && (
                                <ValidationErrors validationErrors={validationErrors}/>
                            )}
                        </div>
                        <Button variant='contained' disabled={disabled} onClick={informCps}>
                            {submitted && <Spinner className='h-6 mr-2 text-inherit'/>}
                            Volgende stap
                            <ArrowRight className="ml-2 h-4 w-4"/>
                        </Button>
                    </ThemeProvider>
                </Card>
            </TaskBody>
        </>
    )
}

