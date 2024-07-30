import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import Combobox from '@/components/ui/combobox'
import MultipleSelector from '@/components/ui/multipleselect'
import useEnrollment from '@/lib/store/enrollment-store'
import React, { useEffect } from 'react'

export default function ConditionSelector() {

    const { conditions, relevantConditions, primaryCondition, setRelevantConditions, setPrimaryCondition } = useEnrollment()

    useEffect(() => {
        console.log("Relevant conditions changed: ", JSON.stringify(relevantConditions, undefined, 2))
    }, [relevantConditions])

    useEffect(() => {
        console.log("Primary condition changed: ", JSON.stringify(primaryCondition, undefined, 2))
    }, [primaryCondition])

    const records = conditions?.map((condition) => {
        return {
            value: condition.id || "no-id",
            label: condition.code?.text || "no-text"
        }
    })

    return (
        <Card>
            <CardHeader>
                <CardTitle>Conditions</CardTitle>
            </CardHeader>
            <CardContent>
                <div className='flex flex-col gap-3'>

                    <div className='font-bold'>Primary Condition</div>
                    <Combobox className='w-full' records={records} onChange={(value) => {
                        console.log("Selected: ", value)
                        if (!value) {
                            setPrimaryCondition(undefined)
                            return
                        }

                        const selectedCondition = conditions?.find(filterCondition => filterCondition.id === value)

                        if (selectedCondition) setPrimaryCondition(selectedCondition)
                    }} />

                    <div className='font-bold'>Relevant Condition(s)</div>
                    <MultipleSelector
                        placeholder='Select one or more relevant conditions'
                        defaultOptions={records}
                        options={records}
                        onChange={(selectedOptions) => {
                            const selectedConditions = selectedOptions
                                .map(option => conditions?.find(filterCondition => filterCondition.id === option.value))
                                .filter(condition => condition !== undefined)

                            if (selectedConditions) {
                                setRelevantConditions(selectedConditions)
                            }
                        }}
                        emptyIndicator={<>No Conditions found for Patient</>}
                    />
                </div>
            </CardContent>
        </Card>
    )
}
