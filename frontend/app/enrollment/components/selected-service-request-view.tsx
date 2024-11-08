import React from 'react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import useEnrollmentStore from '@/lib/store/enrollment-store'
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip'
import { InfoCircledIcon } from '@radix-ui/react-icons'
import OrganizationLabel from "@/app/enrollment/components/organization-label";

export default function SelectedServiceRequestView() {

    const { serviceRequest } = useEnrollmentStore()

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
                    <div className='font-bold'>Task:</div>
                    <div className='col-span-3'>{serviceRequest?.code?.coding?.[0].display || "unknown"}</div>
                    <div className='font-bold'>Sent to:</div>
                    <div className='col-span-3'><OrganizationLabel reference={serviceRequest?.performer?.[0]} /></div>
                </div>
            </CardContent>
        </Card>
    )
}
