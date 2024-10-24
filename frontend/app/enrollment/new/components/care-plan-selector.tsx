import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import Combobox from '@/components/ui/combobox'
import useEnrollment from '@/lib/store/enrollment-store'
import { CheckIcon, PlusIcon } from '@radix-ui/react-icons'
import React, { useState } from 'react'

export default function CarePlanSelector() {
    //TODO: store cp refs instead of requesting all from CPS, as they can be shared accross multiple CPSs

    const { carePlans, selectedCarePlan, setSelectedCarePlan } = useEnrollment()
    const [shouldCreateNewCarePlan, setShouldCreateNewCarePlan] = useState(false)

    const records = carePlans?.map((carePlan) => ({
        value: carePlan.id || "no-id",
        label: carePlan.title || "no-title"
    }))

    return (
        <Card>
            <CardHeader>
                <CardTitle>CarePlan</CardTitle>
                <CardDescription>Select an existing CarePlan or ask the CarePlan Service to create a new CarePlan</CardDescription>
            </CardHeader>
            <CardContent className='flex flex-col gap-3'>
                <Combobox disabled={shouldCreateNewCarePlan} className='w-full' records={records} selectedValue={selectedCarePlan?.id} onChange={(value) => {
                    const carePlan = carePlans?.find((carePlan) => carePlan.id === value)
                    setSelectedCarePlan(carePlan)
                }} />
                {shouldCreateNewCarePlan ? (
                    <Button onClick={() => setShouldCreateNewCarePlan(false)}>
                        <CheckIcon className="mr-2 h-4 w-4" /> Creating new CarePlan
                    </Button>
                ) : (
                    <Button variant="outline" onClick={() => setShouldCreateNewCarePlan(true)}>
                        <PlusIcon className="mr-2 h-4 w-4" /> New CarePlan
                    </Button>
                )}
            </CardContent>
        </Card>
    )
}
