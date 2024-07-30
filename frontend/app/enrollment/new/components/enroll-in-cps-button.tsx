import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle, DialogTrigger } from '@/components/ui/dialog'
import useCpsClient from '@/hooks/use-cps-client'
import { getCarePlan, getBsn, getTask } from '@/lib/fhirUtils'
import useEnrollment from '@/lib/store/enrollment-store'
import { CarePlan, CarePlanActivity, Condition, Task } from 'fhir/r4'
import React, { useEffect, useState } from 'react'
import { toast } from "sonner"
import JsonView from 'react18-json-view';
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
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

    const { patient, selectedCarePlan, shouldCreateNewCarePlan, primaryCondition, relevantConditions, serviceRequest } = useEnrollment()
    const [disabled, setDisabled] = useState(false)
    const [submitted, isSubmitted] = useState(false)

    const cpsClient = useCpsClient()

    useEffect(() => {
        setDisabled(!primaryCondition || (!selectedCarePlan && !shouldCreateNewCarePlan))
    }, [primaryCondition, selectedCarePlan, shouldCreateNewCarePlan])

    const informCps = async () => {
        let carePlan = selectedCarePlan

        if (shouldCreateNewCarePlan) {
            carePlan = await createNewCarePlan() as CarePlan
        }

        if (!carePlan || !primaryCondition) {
            toast.error("Error: Something went wrong with CarePlan creation")
            throw new Error("Something went wrong with CarePlan creation")
        }

        const task = await createTask(carePlan, primaryCondition)

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
    }

    const createNewCarePlan = async () => {
        if (!cpsClient) {
            toast.error("Error: CarePlanService not found")
            throw new Error("No CPS client found")
        }
        if (!patient || !primaryCondition || !serviceRequest) {
            toast.error("Error: Missing required items for CarePlan creation")
            throw new Error("Missing required items for CarePlan creation")
        }

        const carePlan = getCarePlan(patient, primaryCondition, relevantConditions);

        return await cpsClient.create({ resourceType: 'CarePlan', body: carePlan });
    }

    const createTask = async (carePlan: CarePlan, primaryCondition: Condition) => {
        if (!cpsClient) {
            toast.error("Error: CarePlanService not found")
            throw new Error("No CPS client found")
        }
        if (!patient || !primaryCondition || !serviceRequest || !carePlan) {
            toast.error("Error: Missing required items for Task creation")
            throw new Error("Missing required items for Task creation")
        }

        const task = getTask(carePlan, serviceRequest, primaryCondition)
        return await cpsClient.create({ resourceType: 'Task', body: task });
    }

    return (
        <Button disabled={disabled} onClick={informCps} > Proceed</Button >
    )


}