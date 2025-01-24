"use client"
import useCpsClient from '@/hooks/use-cps-client'
import useEhrClient from '@/hooks/use-ehr-fhir-client'
import { findInBundle, getBsn, constructTaskBundle } from '@/lib/fhirUtils'
import useEnrollment from '@/lib/store/enrollment-store'
import { Bundle, Condition, PractitionerRole } from 'fhir/r4'
import React, { useEffect, useState } from 'react'
import { toast } from "sonner"
import { useRouter } from 'next/navigation'
import { ArrowRight, LoaderIcon } from 'lucide-react'
import { Spinner } from '@/components/spinner'
import { Button, createTheme, ThemeProvider } from '@mui/material'

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
export default function EnrollInCpsButton({ className }: Props) {

    const { patient, selectedCarePlan, taskCondition, practitionerRole, serviceRequest, loading, launchContext } = useEnrollment()
    const [disabled, setDisabled] = useState(false)
    const [submitted, setSubmitted] = useState(false)

    const router = useRouter()
    const cpsClient = useCpsClient()
    const ehrClient = useEhrClient()

    useEffect(() => {
        setDisabled(submitted || !taskCondition || loading)
    }, [taskCondition, selectedCarePlan, submitted, loading])

    const theme = createTheme({
        palette: {
            primary: {
                main: '#1c6268',
            },
        },
    });

    const informCps = async () => {
        setSubmitted(true)
        if (!taskCondition) {
            toast.error("Error: Something went wrong with CarePlan creation", { richColors: true })
            throw new Error("Something went wrong with CarePlan creation")
        }

        const taskBundle = await createTask(taskCondition, practitionerRole)
        const task = findInBundle('Task', taskBundle as Bundle);

        if (!task) {
            toast.error("Error: Something went wrong with Task creation", { richColors: true })
            throw new Error("Something went wrong with Task creation")
        }

        router.push(`/enrollment/task/${task.id}`)
    }

    const createTask = async (taskCondition: Condition, practitionerRole?: PractitionerRole) => {
        if (!cpsClient || !ehrClient) {
            toast.error("Error: CarePlanService not found", { richColors: true })
            throw new Error("No CPS client found")
        }
        if (!patient || !getBsn(patient) || !taskCondition || !serviceRequest) {
            toast.error("Error: Missing required items for Task creation", { richColors: true })
            throw new Error("Missing required items for Task creation")
        }

        var taskBundle: Bundle & { type: "transaction"; };

        try {
            taskBundle = constructTaskBundle(serviceRequest, taskCondition, patient, practitionerRole, launchContext?.taskIdentifier);
        } catch (error) {
            console.debug("Error constructing taskBundle");
            console.error(error);
            const msg = `Failed to construct Task Bundle. Error message: ${JSON.stringify(error) ?? "Not error message found"}`;
            toast.error(msg, { richColors: true });
            throw new Error(msg);
        }

        try {
            return await cpsClient.transaction({ body: taskBundle });
        } catch (error) {
            console.debug("Error posting Bundle", taskBundle);
            console.error(error);
            const msg = `Failed to execute Task Bundle. Error message: ${JSON.stringify(error) ?? "Not error message found"}`;
            toast.error(msg, { richColors: true });
            throw new Error(msg);
        }
    }

    return (

        <ThemeProvider theme={theme}>
            <Button variant='contained' disabled={disabled} onClick={informCps}>
                {submitted && <Spinner className='h-6 mr-2 text-inherit' />}
                Volgende stap
                <ArrowRight className="ml-2 h-4 w-4" />
            </Button>
        </ThemeProvider>
    )
}