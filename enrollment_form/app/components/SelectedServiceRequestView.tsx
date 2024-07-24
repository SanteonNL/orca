"use client"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import React, { useEffect, useState } from 'react'
import useEnrollmentStore from '../store/enrollmentStore'

export default function SelectedServiceRequestView() {

    const { serviceRequest, fetchServiceRequest } = useEnrollmentStore()


    useEffect(() => {
        if (!serviceRequest) fetchServiceRequest()
    }, [serviceRequest, fetchServiceRequest])


    return (
        <Card className='m-5'>
            <CardHeader>
                <CardTitle>Order informatie</CardTitle>
            </CardHeader>
            <CardContent>
                <div className='grid grid-cols-4 gap-4'>
                    <div><b>Order type: </b></div>
                    <div className='col-span-3'>{serviceRequest?.code?.coding?.[0].display || "unknown"}</div>
                    <div><b>Executed by: </b></div>
                    <div className='col-span-3'>{serviceRequest?.performer?.[0].display || "unknown"}</div>
                </div>
            </CardContent>
        </Card>
    )
}
