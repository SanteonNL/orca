import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import Combobox from '@/components/ui/combobox'
import useEnrollment from '@/lib/store/enrollment-store'
import { CheckIcon, PlusIcon } from '@radix-ui/react-icons'
import React from 'react'

export default function CarePlanSelector() {

    const { carePlans, shouldCreateNewCarePlan: createNewCarePlan, setSelectedCarePlan, setShouldCreateNewCarePlan: setCreateNewCarePlan } = useEnrollment()

    const records = carePlans?.map((carePlan) => ({
        value: carePlan.id || "no-id",
        label: carePlan.title || "no-title"
    }))

    return (
        <Card>
            <CardHeader>
                <CardTitle>CarePlan</CardTitle>
            </CardHeader>
            <CardContent className='flex flex-col gap-3'>
                <Combobox disabled={createNewCarePlan} className='w-full' records={records} onChange={(value) => {
                    const carePlan = carePlans?.find((carePlan) => carePlan.id === value)
                    if (carePlan) setSelectedCarePlan(carePlan)
                }} />
                {createNewCarePlan ? (
                    <Button onClick={() => setCreateNewCarePlan(false)}>
                        <CheckIcon className="mr-2 h-4 w-4" /> Creating new CarePlan
                    </Button>
                ) : (
                    <Button variant="outline" onClick={() => setCreateNewCarePlan(true)}>
                        <PlusIcon className="mr-2 h-4 w-4" /> New CarePlan
                    </Button>
                )}
            </CardContent>
        </Card>
    )
}
