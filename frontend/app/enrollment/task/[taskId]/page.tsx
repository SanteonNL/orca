"use client"
import { Step, StepItem, Stepper, useStepper } from '@/components/stepper'
import useTaskProgressStore from '@/lib/store/task-progress-store'
import { useParams } from 'next/navigation'
import QuestionnaireRenderer from '../../components/questionnaire-renderer'
import { useEffect, useState } from 'react'
import { Spinner } from '@/components/spinner'
import StepperFooter from '../components/stepper-footer'
import { Task } from 'fhir/r4'
import EnrollmentDetailsView from '../../components/enrollment-details-view'

export default function TaskEnrollmentView() {

    const { taskId } = useParams()
    const { task, loading, initialized, setSelectedTaskId, subTasks, taskToQuestionnaireMap } = useTaskProgressStore()
    const [steps, setSteps] = useState<StepItem[]>([
        { label: "Awaiting Confirmation", description: "Checking if more information is needed..." },
        { label: "Completion", description: "Completion overview" },
    ])
    const [content, setContent] = useState<JSX.Element[]>([
        <>
            <Spinner />
        </>,
        <>
            <EnrollmentDetailsView key="enrollment-status-component" />
            <StepperFooter />
        </>,
    ])

    useEffect(() => {
        setSelectedTaskId(taskId as string)
    }, [setSelectedTaskId, taskId])


    // This useEffect is responsible for setting the steps and content of the stepper based on the subtasks
    useEffect(() => {

        if (!subTasks || !taskToQuestionnaireMap) return

        setSteps([
            ...subTasks.map((task: Task) => {
                return { label: taskToQuestionnaireMap[task.id || ""]?.title || task.id, description: "Subtask Questionnaire" }
            }),
            { label: "Completion", description: "Completion overview" },
        ])

        setContent([
            ...subTasks.map((task: Task) => {
                return (
                    <QuestionnaireRenderer
                        key={task.id}
                        questionnaire={taskToQuestionnaireMap[task.id || ""]}
                        inputTask={task}
                    />
                )
            }),
            <>
                <EnrollmentDetailsView key="enrollment-status-component" />
                <StepperFooter />
            </>
        ])
    }, [subTasks, taskToQuestionnaireMap])


    if (loading || !initialized) return <Spinner />
    if (!task) return <>Failed to find Task, cannot continue!</>
    return (
        <Stepper className='mb-12' initialStep={0} steps={steps} onClickStep={(step, setStep) => setStep(step)}>
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