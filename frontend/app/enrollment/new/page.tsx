"use client"
import React from 'react'
import useEnrollmentStore from '@/lib/store/enrollment-store'
import { ChevronRight, LoaderIcon } from 'lucide-react'
import { Card, CardContent, CardHeader } from '@/components/ui/card'
import OrganizationLabel from '../components/organization-label'
import EnrollInCpsButton from './components/enroll-in-cps-button'


export default function ConfirmDataPreEnrollment() {

  const { serviceRequest, taskCondition, patient, loading } = useEnrollmentStore()

  return (
    <div className="w-screen">
      <div className="max-w-3xl mx-auto">
        <nav className="flex items-center space-x-2 text-sm py-6">
          <span className="font-medium">Verzoek controleren</span>
          <ChevronRight className="h-4 w-4 text-muted-foreground" />
          <span className="text-muted-foreground">Thuismonitoring instellen</span>
        </nav>
      </div>
      <div className="h-px bg-gray-200 mb-2 w-[calc(100%+3rem)] -mx-6"></div>
      <div className="max-w-3xl mx-auto">
        <Card className="border-0 shadow-none px-0">
          <CardHeader className="px-0">
            <h1 className="text-2xl font-medium">Verzoek controleren</h1>
            <p className="text-muted-foreground mt-2">
              Klopt alles? Zo nee: pas het aan in het EPD.
            </p>
          </CardHeader>
          <CardContent className="space-y-6 px-0">
            {loading
              ? (
                <div className="flex items-center justify-center h-32">
                  <LoaderIcon className="h-8 w-8 text-primary animate-spin" />
                </div>
              )
              : (
                <div className="grid grid-cols-[1fr,2fr] gap-y-4">
                  <div className="text-gray-700 font-medium">Patient:</div>
                  <div className="font-bold">{patient?.name?.[0].text || "Unknown"}</div>

                  <div className="text-gray-700 font-medium">Verzoek:</div>
                  <div className="font-bold">{serviceRequest?.code?.coding?.[0].display || "Unknown"}</div>

                  <div className="text-gray-700 font-medium">Diagnose:</div>
                  <div className="font-bold">{taskCondition?.code?.text || taskCondition?.code?.coding?.[0].display || "Unknown"}</div>

                  <div className="text-gray-700 font-medium">Uitvoerende organisatie:</div>
                  <div className="font-bold">
                    <OrganizationLabel reference={serviceRequest?.performer?.[0]} />
                  </div>
                </div>
              )}
          </CardContent>
          <EnrollInCpsButton className='mt-5' />
        </Card>
      </div>
    </div >
  )
}

