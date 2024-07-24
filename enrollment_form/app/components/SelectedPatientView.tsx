"use client"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import React, { useEffect, useState } from 'react'
import useEnrollmentStore from '../store/enrollmentStore'

export default function SelectedPatientView() {

    const { patient, fetchPatient } = useEnrollmentStore()


    useEffect(() => {
        if (!patient) fetchPatient()
    }, [patient, fetchPatient])


    return (
        <Card className='m-5'>
            <CardHeader>
                <CardTitle>Patient</CardTitle>
                <CardDescription>Selected patient</CardDescription>
            </CardHeader>
            <CardContent>
                Patient: {patient?.name?.[0]?.text || "unknown"}
            </CardContent>
        </Card>
    )
}
