import React from 'react'
import { Card, CardContent, CardHeader } from '@/components/ui/card'
import EnrollInCpsButton from './components/enroll-in-cps-button'
import EnrollmentDetails from './components/enrollment-details'

export default function ConfirmDataPreEnrollment() {

  return (
    <Card className="border-0 shadow-none px-0">
      <CardHeader className="px-0">
        <p className="text-muted-foreground">
          Indien het verzoek niet klopt, pas het dan aan in het EPD.
        </p>
      </CardHeader>
      <CardContent className="space-y-6 px-0">
        <EnrollmentDetails />
      </CardContent>
      <EnrollInCpsButton className='mt-5' />
    </Card>
  )
}

