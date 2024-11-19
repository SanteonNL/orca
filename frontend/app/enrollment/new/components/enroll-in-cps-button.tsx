import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle, DialogTrigger } from '@/components/ui/dialog'
import useCpsClient from '@/hooks/use-cps-client'
import useEhrClient from '@/hooks/use-ehr-fhir-client'
import { getBsn, getCarePlan, getTaskBundle } from '@/lib/fhirUtils'
import useEnrollment from '@/lib/store/enrollment-store'
import { CarePlan, Condition, Questionnaire, ServiceRequest } from 'fhir/r4'
import React, { useEffect, useState } from 'react'
import { toast } from "sonner"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { useRouter } from 'next/navigation'
import JsonView from 'react18-json-view';
import 'react18-json-view/src/style.css';
import { Spinner } from '@/components/spinner'

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

    const { patient, selectedCarePlan, taskCondition, serviceRequest } = useEnrollment()
    const [disabled, setDisabled] = useState(false)
    const [submitted, setSubmitted] = useState(false)

    const router = useRouter()
    const cpsClient = useCpsClient()
    const ehrClient = useEhrClient()

    useEffect(() => {
        setDisabled(submitted || !taskCondition)
    }, [taskCondition, selectedCarePlan, submitted])

    const informCps = async () => {
        setSubmitted(true)
        if (!taskCondition) {
            toast.error("Error: Something went wrong with CarePlan creation", { richColors: true })
            throw new Error("Something went wrong with CarePlan creation")
        }

        const task = await createTask(taskCondition)

        toast.success("Enrollment successfully sent to filler", {
            closeButton: true,
            important: true,
            description: new Date().toLocaleDateString(undefined, {
                year: 'numeric',
                month: 'long',
                day: 'numeric',
                hour: '2-digit',
                minute: '2-digit'
            }),
            action: (
                <Dialog>
                    <DialogTrigger asChild>
                        <Button variant="outline">View Task</Button>
                    </DialogTrigger>
                    <DialogContent className="min-w-[90vw] max-h-[90vh] overflow-y-scroll">
                        <DialogHeader>
                            <DialogTitle>Created Task</DialogTitle>
                            <DialogDescription>
                                See the results of the created Task below
                            </DialogDescription>
                        </DialogHeader>
                        <div className='overflow-auto'>
                            <JsonView src={task} collapsed={2} />
                        </div>
                    </DialogContent>
                </Dialog >
            )
        })

        router.push(`/enrollment/task/${task.id}`)
    }

    const forwardServiceRequest = async () => {
        if (!serviceRequest) {
            toast.error("Error: Missing ServiceRequest - Cannot forward to CPS", { richColors: true })
            throw new Error("Missing ServiceRequest - Cannot forward to CPS")
        }

        if (!cpsClient) {
            toast.error("Error: CarePlanService not found", { richColors: true })
            throw new Error("No CPS client found")
        }

        try {
            // Clean up the ServiceRequest by removing relative references - the CPS won't understand them
            // TODO: Properly fix with with bundle refs in INT-288
            const cleanServiceRequest = { ...serviceRequest };
            if (cleanServiceRequest.subject?.reference) {
                delete cleanServiceRequest.subject.reference;
            }
            if (cleanServiceRequest.requester?.reference) {
                delete cleanServiceRequest.requester.reference;
            }
            if (cleanServiceRequest.performer?.[0]?.reference) {
                delete cleanServiceRequest.performer[0].reference;
            }

            return await cpsClient.create({ resourceType: 'ServiceRequest', body: cleanServiceRequest }) as ServiceRequest
        } catch (error) {
            const msg = `Failed to forward ServiceRequest. Error message: ${error ?? "No error message found"}`
            toast.error(msg, { richColors: true })
            throw new Error(msg)
        }
    }

    const createTask = async (taskCondition: Condition) => {
        if (!cpsClient || !ehrClient) {
            toast.error("Error: CarePlanService not found", { richColors: true })
            throw new Error("No CPS client found")
        }
        if (!patient || !getBsn(patient) || !taskCondition || !serviceRequest) {
            toast.error("Error: Missing required items for Task creation", { richColors: true })
            throw new Error("Missing required items for Task creation")
        }

        const forwardedServiceRequest = await forwardServiceRequest();

        const taskBundle = getTaskBundle(forwardedServiceRequest, taskCondition, patient);

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
        <Button className={className} disabled={disabled} onClick={informCps}>
            {submitted ? <Spinner className='mr-5 text-white' /> : null}
            <span className='mr-1'>{submitted ? "Sending" : "Send"}</span>
            Task
            {serviceRequest?.performer?.[0].display && <span className='ml-1'>to {serviceRequest.performer[0].display}</span>}
        </Button>
    )
}