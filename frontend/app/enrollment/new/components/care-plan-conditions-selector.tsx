import { Label } from '@/components/ui/label'
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

        <div>
            <Label>Conditions</Label>
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
    )
}
