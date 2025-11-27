"use client"
import {findInBundle, getPatientIdentifier, constructTaskBundle} from '@/lib/fhirUtils'
import useEnrollment from '@/app/hooks/enrollment-hook'
import {Bundle, Coding, Condition, OperationOutcome, PractitionerRole} from 'fhir/r4'
import React, {useEffect, useState} from 'react'
import {toast} from "sonner"
import {useRouter} from 'next/navigation'
import {ArrowRight} from 'lucide-react'
import {Spinner} from '@/components/spinner'
import {Button, ThemeProvider} from '@mui/material'
import {defaultTheme} from "@/app/theme"
import ValidationErrors from './validation-errors'
import { useClients, useLaunchContext } from '@/app/hooks/context-hook'

interface Props {
    className?: string
}

/**
 * This button informs the CarePlanService of the new enrollment.
 *
 * It currently always creates a new CarePlan in the CPS
 *
 * @returns
 */
export default function EnrollInCpsButton({className}: Props) {

    const {
        patient,
        taskCondition,
        practitionerRole,
        serviceRequest,
        isLoading,
    } = useEnrollment()

    const { launchContext } = useLaunchContext()
    const { cpsClient } = useClients()

    const [disabled, setDisabled] = useState(false)
    const [submitted, setSubmitted] = useState(false)
    const [error, setError] = useState<string | null>()
    const [validationErrors, setValidationErrors] = useState<Coding[]>()

    const router = useRouter()

    useEffect(() => {
        setDisabled(submitted || !taskCondition || isLoading)
    }, [taskCondition, submitted, isLoading])

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
    )
}