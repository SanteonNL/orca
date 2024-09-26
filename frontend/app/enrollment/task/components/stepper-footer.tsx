"use client"
import { useStepper } from '@/components/stepper'
import { Button } from '@/components/ui/button'
import React, { useEffect, useState } from 'react'

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

    const closePopup = () => {
        window.close();
    };

    const handleNext = () => {
        window.scrollTo({ top: 0, behavior: 'smooth' })
        nextStep();
    }

    return (
        <>
            {hasCompletedAllSteps && (
                <div className="h-40 flex items-center justify-center my-2 border bg-secondary text-primary rounded-md">
                    <h1 className="text-xl">Enrollment completed! 🎉</h1>
                </div>
            )}
            <div className="w-full flex justify-end gap-2 mt-2">
                {isLastStep ? (
                    <Button size="sm" onClick={closePopup}>Close</Button>
                ) : (
                    <Button size="sm" onClick={handleNext}>
                        {isOptionalStep ? "Skip" : "Next"}
                    </Button>
                )}
            </div>
        </>
    )
}