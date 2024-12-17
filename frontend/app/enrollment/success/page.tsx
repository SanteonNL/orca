"use client"
import React from 'react'
import useEnrollmentStore from '@/lib/store/enrollment-store'
import organizationName from '@/lib/fhirUtils'

export default function EnrollmentSuccessPage() {

    const { serviceRequest, taskCondition } = useEnrollmentStore()

    return (
        <div className='w-[568px]'>
            Het verzoek om {serviceRequest?.code?.coding?.[0].display || "Unknown"} voor {taskCondition?.code?.text || taskCondition?.code?.coding?.[0].display || "Unknown"} uit te voeren is correct verzonden en door {organizationName(serviceRequest?.performer?.[0])} geaccepteerd. U kunt nu dit scherm sluiten.
        </div>
    )
}
