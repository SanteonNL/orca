"use client"
import { Step, StepItem, Stepper, useStepper } from '@/components/stepper'
import useTaskProgressStore from '@/lib/store/task-progress-store'
import { useParams } from 'next/navigation'
import TaskStatus from '../components/task-status'
import QuestionnaireRenderer from '../../components/questionnaire-renderer'
import { useEffect } from 'react'
import { Spinner } from '@/components/spinner'
import StepperFooter from '../components/stepper-footer'

export default function TaskEnrollmentView() {

    const { taskId } = useParams()
    const { task, loading, initialized, setSelectedTaskId } = useTaskProgressStore()
    const { nextStep } = useStepper()

    useEffect(() => {
        setSelectedTaskId(taskId as string)
    }, [])

    const steps = [
        { label: "Task overview", description: "Information sent to the filler" },
        { label: "Extra Information", description: "The filler might request additional information" },
        { label: "Completion", description: "Completion overview" },
    ] satisfies StepItem[]

    const content = [
        <>
            <TaskStatus key="task-status-component" />
            <StepperFooter />
        </>,
        <QuestionnaireRenderer key="questionnaire-renderer" inputTask={task} onSubmit={nextStep} />,
        <>
            <TaskStatus key="task-status-component" />
            <StepperFooter />
        </>,
    ]

    if (loading || !initialized) return <Spinner />
    if (!task) return <>Failed to find Task, cannot continue!</>
    return (
        <Stepper className='mb-12' initialStep={task.status === "accepted" ? 2 : 0} steps={steps}>
            {steps.map((stepProps, index) => {
                return (
                    <Step key={stepProps.label} {...stepProps}>
                        {content[index]}
                    </Step>
                )
            })}
        </Stepper>
    )
}