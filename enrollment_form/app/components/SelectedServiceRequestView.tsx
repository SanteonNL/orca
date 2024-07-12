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
                <CardTitle>ServicieRequest</CardTitle>
                <CardDescription>Selected request</CardDescription>
            </CardHeader>
            <CardContent>
                Condition: {serviceRequest?.reasonReference?.[0]?.display || "unknown"}
            </CardContent>
        </Card>
    )
}
