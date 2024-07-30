"use client"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import React, { useEffect, useState } from 'react'
import useEnrollmentStore from '@/lib/store/enrollment-store'
import Image from 'next/image'
import { getBsn } from '@/lib/fhirUtils'

export default function SelectedPatientView() {

    const { patient } = useEnrollmentStore()
    const bsn = getBsn(patient)

    return (
        <Card className='bg-primary text-gray-100'>
            <CardHeader>
                <CardTitle>Patient Information</CardTitle>
            </CardHeader>
            <CardContent>
                <div className="grid grid-cols-4 gap-4">
                    <div className="col-span-1 bg-gray-100 p-6 rounded-lg shadow-md flex items-center justify-center">
                        <Image
                            src="https://randomuser.me/api/portraits/men/75.jpg"
                            width={144}
                            height={144}
                            alt="User Image"
                            className="w-36 h-38 rounded-full mb-4"
                        />
                    </div>

                    <div className="col-span-3 space-y-4">
                        <div className="bg-gray-100 text-primary p-6 rounded-lg shadow-md">
                            <h3 className="text-xl font-semibold">Name</h3>
                            <p className="text-gray-600">{patient?.name?.[0].text || "Unknown"}</p>
                        </div>
                        <div className="bg-gray-100 text-primary p-6 rounded-lg shadow-md">
                            <h3 className="text-xl font-semibold">BSN</h3>
                            <p className="text-gray-600">{bsn || "Unknown"}</p>
                        </div>
                        <div className="bg-gray-100 text-primary p-6 rounded-lg shadow-md">
                            <h3 className="text-xl font-semibold">Address</h3>
                            <p className="text-gray-600">{patient?.address?.[0].text || "Unknown"}</p>
                        </div>
                    </div>
                </div>
            </CardContent>
        </Card>
    )
}
