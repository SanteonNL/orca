"use client"
import useCpsClient from '@/hooks/use-cps-client'
import {findInBundle, getPatientIdentifier, constructTaskBundle} from '@/lib/fhirUtils'
import useEnrollment from '@/lib/store/enrollment-store'
import {Bundle, Condition, OperationOutcome, PractitionerRole} from 'fhir/r4'
import React, {useEffect, useState} from 'react'
import {toast} from "sonner"
import {useRouter} from 'next/navigation'
import {ArrowRight, LoaderIcon} from 'lucide-react'
import {Spinner} from '@/components/spinner'
import {Button, createTheme, ThemeProvider} from '@mui/material'
import {defaultTheme} from "@/app/theme";

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
        selectedCarePlan,
        taskCondition,
        practitionerRole,
        serviceRequest,
        loading,
        launchContext
    } = useEnrollment()
    const [disabled, setDisabled] = useState(false)
    const [submitted, setSubmitted] = useState(false)
    const [error, setError] = useState<string>()

    const router = useRouter()
    const cpsClient = useCpsClient()

    useEffect(() => {
        setDisabled(submitted || !taskCondition || loading)
    }, [taskCondition, selectedCarePlan, submitted, loading])

    const informCps = async () => {
        setSubmitted(true)
        setError(undefined)

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
        if (!cpsClient) {
            throw new Error("CarePlanService not found")
        }
        if (!patient || !getPatientIdentifier(patient) || !taskCondition || !serviceRequest) {
            throw new Error("Missing required items for Task creation")
        }

        let taskBundle: Bundle & { type: "transaction"; };

        try {
            taskBundle = constructTaskBundle(serviceRequest, taskCondition, patient, practitionerRole, launchContext?.taskIdentifier);
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
                const errorMessage = operationOutcome?.issue[0].diagnostics || error.message || "Bad request";
                throw new Error(`Validation error: ${errorMessage}`);
            }

            throw new Error(`Failed to execute Task Bundle: ${error.message || "Unknown error"}`);
        }
    }

    return (
        <ThemeProvider theme={defaultTheme}>
            <div>
                {error && (
                    <div className='text-red-500 mb-2'>
                        {error}
                        <p className='text-muted-foreground mb-2'>
                            Indien het verzoek niet klopt, pas het dan aan in het EPD.
                        </p>
                    </div>
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