import { Input } from '@/components/ui/input'
import React from 'react'
import CarePlanConditionSelector from './care-plan-conditions-selector'
import useEnrollment from '@/lib/store/enrollment-store'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Label } from '@/components/ui/label'

export default function NewCarePlanInformation() {

    const { newCarePlanName, setNewCarePlanName } = useEnrollment()

    return (

        <Card className='m-6 mt-0'>
            <CardHeader>
                <CardTitle>New CarePlan Information</CardTitle>
            </CardHeader>
            <CardContent>

                <div className='flex flex-col gap-3 space-y-1.5 p-6'>
                    <div>
                        <Label htmlFor="cp-name">CarePlan name</Label>
                        <Input id="cp-name" type="text" placeholder="CarePlan for COPD worsening prevention" value={newCarePlanName} onChange={(e) => setNewCarePlanName(e.target.value)} />
                    </div>
                    <CarePlanConditionSelector />
                </div>
            </CardContent>
        </Card>
    )
}