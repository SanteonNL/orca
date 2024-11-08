"use client"
import { Spinner } from '@/components/spinner'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { getTaskPerformer } from '@/lib/fhirUtils'
import useTaskProgressStore from '@/lib/store/task-progress-store'
import { Reference } from 'fhir/r4'
import React, { useEffect, useState } from 'react'
import SelectedPatientView from '../../components/selected-patient-view'
import OrganizationLabel from "@/app/enrollment/components/organization-label";

export default function TaskStatus() {

  const { task, loading } = useTaskProgressStore()
  const [performer, setPerformer] = useState<Reference>()

  useEffect(() => {
    setPerformer(
      getTaskPerformer(task)
    )
  }, [task])

  return (
    <Card className='mt-5 bg-primary text-gray-100'>
      <CardHeader>
        <CardTitle>Task information</CardTitle>
        <CardDescription>The following Task has been sent</CardDescription>
      </CardHeader>
      <CardContent>
        <>
          <div className='grid grid-cols-4 gap-4 mb-5'>
            <div className='font-bold'>Task focus:</div>
            <div className='col-span-3'>
              {loading}
              {loading ? <Spinner /> : `${task?.focus?.display}` || "unknown"}
            </div>
            <div className='font-bold'>Status:</div>
            <div className='col-span-3'>
              {loading ? <Spinner /> : task?.status || "unknown"}
            </div>
            <div className='font-bold'>Sent to:</div>
            <div className='col-span-3'>
              {loading ? <Spinner /> : <OrganizationLabel reference={performer} />}
            </div>
          </div>
          <SelectedPatientView />
        </>
      </CardContent>
    </Card>
  )
}
