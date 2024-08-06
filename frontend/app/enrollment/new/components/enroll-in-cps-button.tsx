import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle, DialogTrigger } from '@/components/ui/dialog'
import useCpsClient from '@/hooks/use-cps-client'
import { getCarePlan, getTask } from '@/lib/fhirUtils'
import useEnrollment from '@/lib/store/enrollment-store'
import { CarePlan, Condition } from 'fhir/r4'
import React, { useEffect, useState } from 'react'
import { toast } from "sonner"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { useRouter } from 'next/navigation'
import JsonView from 'react18-json-view';
import 'react18-json-view/src/style.css';

/**
 * This button informs the CarePlanService of the new enrollment.
 * 
 * 1. If the CarePlan does not exist yet, it will be created.
 * 2. It will create a Task, referring to the CarePlan and ServiceRequest
 * 
 * @returns 
 */
export default function EnrollInCpsButton() {

    const { patient, selectedCarePlan, shouldCreateNewCarePlan, taskCondition, carePlanConditions, serviceRequest, newCarePlanName } = useEnrollment()
    const [disabled, setDisabled] = useState(false)

    const router = useRouter()
    const cpsClient = useCpsClient()

    useEffect(() => {
        setDisabled(!taskCondition || (!selectedCarePlan && !shouldCreateNewCarePlan) || (shouldCreateNewCarePlan && (!newCarePlanName || !carePlanConditions)))
    }, [taskCondition, selectedCarePlan, shouldCreateNewCarePlan, newCarePlanName, carePlanConditions])

    const informCps = async () => {
        let carePlan = selectedCarePlan

        if (shouldCreateNewCarePlan) {
            carePlan = await createNewCarePlan() as CarePlan
        }

        if (!carePlan || !taskCondition) {
            toast.error("Error: Something went wrong with CarePlan creation", { richColors: true })
            throw new Error("Something went wrong with CarePlan creation")
        }

        const task = await createTask(carePlan, taskCondition)

        toast.success("Enrollment succeeded", {
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
                        <Button variant="outline">View Resources</Button>
                    </DialogTrigger>
                    <DialogContent className="min-w-[90vw] max-h-[90vh] overflow-y-scroll">
                        <DialogHeader>
                            <DialogTitle>Created Resources</DialogTitle>
                            <DialogDescription>
                                See the results of the created CarePlan and Task below
                            </DialogDescription>
                        </DialogHeader>
                        <Tabs className="w-full" defaultValue='careplan'>
                            <TabsList className="grid w-full grid-cols-2">
                                <TabsTrigger value="careplan">CarePlan</TabsTrigger>
                                <TabsTrigger value="task">Task</TabsTrigger>
                            </TabsList>
                            <div className='overflow-auto'>
                                <TabsContent value="careplan">
                                    <JsonView src={carePlan} collapsed={2} />
                                </TabsContent>
                                <TabsContent value="task">
                                    <JsonView src={task} collapsed={2} />
                                </TabsContent>
                            </div>
                        </Tabs>
                    </DialogContent>
                </Dialog>
            )
        })

        router.push(`/enrollment/task/${task.id}`)
    }

    const createNewCarePlan = async () => {
        if (!cpsClient) {
            toast.error("Error: CarePlanService not found", { richColors: true })
            throw new Error("No CPS client found")
        }
        if (!patient || !taskCondition || !serviceRequest || !carePlanConditions) {
            toast.error("Error: Missing required items for CarePlan creation", { richColors: true })
            throw new Error("Missing required items for CarePlan creation")
        }

        const carePlan = getCarePlan(patient, carePlanConditions, newCarePlanName);

        try {
            return await cpsClient.create({ resourceType: 'CarePlan', body: carePlan })
        } catch (error) {
            const msg = `Failed to create CarePlan. Error message: ${error ?? "Not error message found"}`
            toast.error(msg, { richColors: true })
            throw new Error(msg)
        }
    }

    const createTask = async (carePlan: CarePlan, taskCondition: Condition) => {
        if (!cpsClient) {
            toast.error("Error: CarePlanService not found", { richColors: true })
            throw new Error("No CPS client found")
        }
        if (!patient || !taskCondition || !serviceRequest || !carePlan) {
            toast.error("Error: Missing required items for Task creation", { richColors: true })
            throw new Error("Missing required items for Task creation")
        }

        const task = getTask(carePlan, serviceRequest, taskCondition)

        try {
            return await cpsClient.create({ resourceType: 'Task', body: task });
        } catch (error) {
            const msg = `Failed to create Task. Error message: ${error ?? "Not error message found"}`
            toast.error(msg, { richColors: true })
            throw new Error(msg)
        }
    }

    return (
        <Button disabled={disabled} onClick={informCps}>Proceed</Button >
    )


}