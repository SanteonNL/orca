import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import Combobox from '@/components/ui/combobox'
import useEnrollment from '@/lib/store/enrollment-store'
import React from 'react'

export default function TaskConditionSelector() {

    const { patientConditions, taskCondition, setTaskCondition } = useEnrollment()

    const records = patientConditions?.map((condition) => {
        return {
            value: condition.id || "no-id",
            label: condition.code?.text || "no-text"
        }
    })

    return (
        <Card>
            <CardHeader>
                <CardTitle>Task Condition</CardTitle>
                <CardDescription>
                    Select the Condition that should be addressed in this Task
                </CardDescription>
            </CardHeader>
            <CardContent>
                <div className='flex flex-col gap-3'>
                    <div className='font-bold'>Task Condition</div>
                    <Combobox className='w-full' records={records} selectedValue={taskCondition?.id} onChange={(value) => {
                        if (!value) {
                            setTaskCondition(undefined)
                            return
                        }

                        const selectedCondition = patientConditions?.find(filterCondition => filterCondition.id === value)

                        if (selectedCondition) setTaskCondition(selectedCondition)
                    }} />
                </div>
            </CardContent>
        </Card>
    )
}
