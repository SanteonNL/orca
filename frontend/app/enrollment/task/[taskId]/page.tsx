"use client"
import React, { useEffect } from 'react'
import useTaskProgressStore from '@/lib/store/task-progress-store'
import { useParams } from 'next/navigation'
import Loading from '@/app/enrollment/loading'
import QuestionnaireRenderer from '../../components/questionnaire-renderer'
import organizationName from '@/lib/fhirUtils'

export default function EnrollmentTaskPage() {
    const { taskId } = useParams()
    const { task, loading, initialized, setSelectedTaskId, subTasks, taskToQuestionnaireMap } = useTaskProgressStore()

    useEffect(() => {
        if (taskId) {
            //TODO: Currently we only have one Questionnaire per enrollment flow. But we support multiple. The UX for multiple still needs to be made. When it's there, this is the place to add it
            const selectedTaskId = Array.isArray(taskId) ? taskId[0] : taskId;
            setSelectedTaskId(selectedTaskId);
        }
    }, [taskId, setSelectedTaskId])

    if (loading || !initialized) return <Loading />

    const serviceName = task?.focus?.display || "Unknown"
    const conditionName = task?.reasonCode?.coding?.[0].display || "Unknown"
    const taskDate = task?.meta?.lastUpdated ? new Date(task.meta.lastUpdated).toLocaleDateString("nl-NL") : "Onbekend"

    const StatusWrapper = ({ children }: { children: React.ReactNode }) => <div className='w-[568px]'>{children}</div>

    switch (task?.status) {
        case "accepted":
            return <StatusWrapper>Het verzoek om {serviceName} voor {conditionName} uit te voeren, is door {organizationName(task.owner)} op {taskDate} geaccepteerd. Uitvoering is nog niet gestart.</StatusWrapper>
        case "completed":
            return <StatusWrapper>De uitvoering van {serviceName} voor {conditionName} is door {organizationName(task.owner)} afgerond op {taskDate}.</StatusWrapper>
        case "cancelled":
            return <StatusWrapper>Het verzoek om {serviceName} voor {conditionName} uit te voeren, is op {taskDate} geannuleerd door {organizationName(task.owner)}.</StatusWrapper>
        case "failed":
            return <StatusWrapper>Het verzoek om {serviceName} voor {conditionName} uit te voeren, is sinds {taskDate} gemarkeerd als &quot;mislukt&quot; door {organizationName(task.owner)}.</StatusWrapper>
        case "in-progress":
            return <StatusWrapper>Het verzoek om {serviceName} voor {conditionName} uit te voeren, wordt momenteel (sinds {taskDate}) uitgevoerd door {organizationName(task.owner)}.</StatusWrapper>
        case "on-hold":
            return <StatusWrapper>Het verzoek om {serviceName} voor {conditionName} uit te voeren, is sinds {taskDate} gepauseerd door {organizationName(task.owner)}.</StatusWrapper>
        case "requested":
            return <StatusWrapper>Het verzoek om {serviceName} voor {conditionName} is verstuurd naar {organizationName(task.owner)}, maar nog niet ontvangen.</StatusWrapper>
        case "received":
            if (!taskToQuestionnaireMap || !subTasks?.[0]?.id || !taskToQuestionnaireMap[subTasks[0].id]) {
                return <>Task is ontvangen, maar er ontbreekt informatie.</>
            }
            return <QuestionnaireRenderer
                questionnaire={taskToQuestionnaireMap[subTasks[0].id]}
                inputTask={subTasks[0]}
            />
        case "rejected":
            return <StatusWrapper>Het verzoek om {serviceName} voor {conditionName} uit te voeren is op {taskDate} afgewezen door {organizationName(task.owner)}.</StatusWrapper>
        default:
            //primary tasks cannot handle Task.stats, for example `ready`
            return <StatusWrapper>Task status {task?.status || "Taak niet gevonden"} is geen valide status voor een enrollment Task.</StatusWrapper>
    }
}
