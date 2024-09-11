"use client"
import { Step, StepItem, Stepper, useStepper } from '@/components/stepper'
import useTaskProgressStore from '@/lib/store/task-progress-store'
import { useParams } from 'next/navigation'
import TaskStatus from '../components/task-status'
import QuestionnaireRenderer from '../../components/questionnaire-renderer'
import { useEffect, useState } from 'react'
import { Spinner } from '@/components/spinner'
import StepperFooter from '../components/stepper-footer'
import useCpsClient from '@/hooks/use-cps-client'
import { Task } from 'fhir/r4'

export default function TaskEnrollmentView() {

    const { taskId } = useParams()
    const { task, loading, initialized, setSelectedTaskId, subTasks, taskToQuestionnaireMap } = useTaskProgressStore()
    const { nextStep } = useStepper()
    const [steps, setSteps] = useState<StepItem[]>([
        { label: "Task overview", description: "Information sent to the filler" },
        { label: "Completion", description: "Completion overview" },
    ])
    const [content, setContent] = useState<JSX.Element[]>([
        <>
            <TaskStatus key="task-status-component" />
            <StepperFooter />
        </>,
        <>
            <TaskStatus key="task-status-component" />
            <StepperFooter />
        </>,
    ])
    const cpsClient = useCpsClient()
    const [error, setError] = useState<string>()

    useEffect(() => {
        setSelectedTaskId(taskId as string)
    }, [])


    // This useEffect is responsible for setting the steps and content of the stepper based on the subtasks
    useEffect(() => {

        if (!subTasks || !taskToQuestionnaireMap) return

        setSteps([
            { label: "Task overview", description: "Information sent to the filler" },
            ...subTasks.map((task: Task) => {
                return { label: taskToQuestionnaireMap[task.id || ""]?.title || task.id, description: "Subtask Questionnaire" }
            }),
            { label: "Completion", description: "Completion overview" },
        ])

        setContent([
            <>
                <TaskStatus key="task-status-component" />
                <StepperFooter />
            </>,
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
                <TaskStatus key="task-status-component" />
                <StepperFooter />
            </>
        ])
    }, [subTasks, taskToQuestionnaireMap])


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