import React from 'react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import useEnrollmentStore from '@/lib/store/enrollment-store'
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip'
import { InfoCircledIcon } from '@radix-ui/react-icons'
import { getBsn } from '@/lib/fhirUtils'
import useTaskProgressStore from '@/lib/store/task-progress-store'
import OrganizationLabel from './organization-label'

export default function EnrollmentDetailsView() {

    const { serviceRequest, taskCondition, patient } = useEnrollmentStore()
    const { task } = useTaskProgressStore()

    return (
        <Card className='my-5 bg-primary text-gray-100'>
            <CardHeader>
                <CardTitle>
                    <div className='flex items-center'>
                        <TooltipProvider>
                            <Tooltip>
                                <TooltipTrigger asChild>
                                    <InfoCircledIcon className='h-6 w-6 cursor-help' />
                                </TooltipTrigger>
                                <TooltipContent>
                                    <p>A Task refers to the outgoing order</p>
                                </TooltipContent>
                            </Tooltip>
                        </TooltipProvider>
                        <p className='ml-2'>Task information</p>
                    </div>
                </CardTitle>
            </CardHeader>
            <CardContent>
                <div className='grid grid-cols-4 gap-4'>
                    <div className='font-bold'>Patient:</div>
                    <div className='col-span-3'>{patient?.name?.[0].text || "Unknown"}</div>
                    <div className='font-bold'>BSN:</div>
                    <div className='col-span-3'>{getBsn(patient) || "Unknown"}</div>
                    <div className='font-bold'>Service:</div>
                    <div className='col-span-3'>{serviceRequest?.code?.coding?.[0].display || "Unknown"}</div>
                    <div className='font-bold'>Care Path:</div>
                    <div className='col-span-3'>{taskCondition?.code?.coding?.[0].display || "Unknown"}</div>
                    <div className='font-bold'>{task?.status ? 'Sent' : 'Send'} to:</div>
                    <div className='col-span-3'>
                        <OrganizationLabel reference={serviceRequest?.performer?.[0]} />
                    </div>
                    {task?.status && (
                        <>
                            <div className='font-bold'>Task status:</div>
                            <div className='col-span-3'>{task?.status}</div>
                        </>
                    )}
                </div>
            </CardContent>
        </Card>
    )
}
