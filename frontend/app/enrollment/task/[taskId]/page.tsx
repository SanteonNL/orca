"use client"
import React, { useEffect } from 'react'
import useTaskProgressStore from '@/lib/store/task-progress-store'
import { useParams } from 'next/navigation'
import Loading from '@/app/enrollment/loading'
import QuestionnaireRenderer from '../../components/questionnaire-renderer'

export default function EnrollmentTaskPage() {
    const { taskId } = useParams()
    const { loading, initialized, setSelectedTaskId, subTasks, taskToQuestionnaireMap } = useTaskProgressStore()

    useEffect(() => {
        if (taskId) {
            //TODO: Currently we only have one Questionnaire per enrollment flow. But we support multiple. The UX for multiple still needs to be made. When it's there, this is the place to add it
            const selectedTaskId = Array.isArray(taskId) ? taskId[0] : taskId;
            setSelectedTaskId(selectedTaskId);
        }
    }, [taskId, setSelectedTaskId])

    if (loading || !initialized) return <Loading />

    if (!subTasks || !subTasks.length || !taskToQuestionnaireMap || !subTasks?.[0].id || !taskToQuestionnaireMap[subTasks[0].id]) {
        return <></>
    }

    return (
        <QuestionnaireRenderer
            questionnaire={taskToQuestionnaireMap[subTasks[0].id]}
            inputTask={subTasks[0]}
        />
    )
}
