import { useStepper } from '@/components/stepper'
import { Button } from '@/components/ui/button'
import React from 'react'

export default function StepperFooter() {
    const {
        nextStep,
        prevStep,
        resetSteps,
        hasCompletedAllSteps,
        isLastStep,
        isOptionalStep,
        isDisabledStep,
    } = useStepper()
    return (
        <>
            {hasCompletedAllSteps && (
                <div className="h-40 flex items-center justify-center my-2 border bg-secondary text-primary rounded-md">
                    <h1 className="text-xl">Enrollment completed! 🎉</h1>
                </div>
            )}
            <div className="w-full flex justify-end gap-2 mt-2">
                {hasCompletedAllSteps ? (
                    <Button size="sm" onClick={resetSteps}>
                        Reset
                    </Button>
                ) : (
                    <>
                        <Button
                            disabled={isDisabledStep}
                            onClick={prevStep}
                            size="sm"
                            variant="secondary"
                        >
                            Prev
                        </Button>
                        <Button size="sm" onClick={nextStep}>
                            {isLastStep ? "Finish" : isOptionalStep ? "Skip" : "Next"}
                        </Button>
                    </>
                )}
            </div>
        </>
    )
}