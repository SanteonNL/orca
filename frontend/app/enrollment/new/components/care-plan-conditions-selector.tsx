import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import MultipleSelector from '@/components/ui/multipleselect'
import useEnrollment from '@/lib/store/enrollment-store'
import React from 'react'

export default function CarePlanConditionSelector() {

    const { patientConditions, setCarePlanConditions } = useEnrollment()

    const records = patientConditions?.map((condition) => {
        return {
            value: condition.id || "no-id",
            label: condition.code?.text || "no-text"
        }
    })

    if (!records) <>No Conditions selected</>

    return (
        <Card>
            <CardHeader>
                <CardTitle>CarePlan Conditions</CardTitle>
                <CardDescription>Select all relevant conditions to this CarePlan</CardDescription>
            </CardHeader>
            <CardContent>
                <div className='flex flex-col gap-3'>
                    <MultipleSelector
                        placeholder='Select one or more relevant conditions'
                        defaultOptions={records}
                        options={records}
                        onChange={(selectedOptions) => {
                            const selectedConditions = selectedOptions
                                .map(option => patientConditions?.find(filterCondition => filterCondition.id === option.value))
                                .filter(condition => condition !== undefined)

                            if (selectedConditions) {
                                setCarePlanConditions(selectedConditions)
                            }
                        }}
                        emptyIndicator={<>No Conditions found for Patient</>}
                    />
                </div>
            </CardContent>
        </Card>
    )
}
