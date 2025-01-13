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

    switch (task?.status) {
        case "accepted":
            return <div className='w-[568px]'>
                Het verzoek om {serviceName} voor {conditionName} uit te voeren, is door {organizationName(task.owner)} op {taskDate} geaccepteerd. Uitvoering is nog niet gestart.
            </div>
        case "completed":
            return <div className='w-[568px]'>
                De uitvoering van {serviceName} voor {conditionName} is door {organizationName(task.owner)} afgerond op {taskDate}.
            </div>
        case "cancelled":
            return <div className='w-[568px]'>
                Het verzoek om {serviceName} voor {conditionName} uit te voeren, is op {taskDate} geannuleerd door {organizationName(task.owner)}.
            </div>
        case "failed":
            return <div className='w-[568px]'>
                Het verzoek om {serviceName} voor {conditionName} uit te voeren, is sinds {taskDate} gemarkeerd als &quot;mislukt&quot; door {organizationName(task.owner)}.
            </div>
        case "in-progress":
            return <div className='w-[568px]'>
                Het verzoek om {serviceName} voor {conditionName} uit te voeren, wordt momenteel (sinds {taskDate}) uitgevoerd door {organizationName(task.owner)}.
            </div>
        case "on-hold":
            return <div className='w-[568px]'>
                Het verzoek om {serviceName} voor {conditionName} uit te voeren, is sinds {taskDate} gepauseerd door {organizationName(task.owner)}.
            </div>
        case "requested":
            return <div className='w-[568px]'>
                Het verzoek om {serviceName} voor {conditionName} is verstuurd naar {organizationName(task.owner)}, maar nog niet ontvangen.
            </div>
        case "received":
            if (!taskToQuestionnaireMap || !subTasks?.[0]?.id || !taskToQuestionnaireMap[subTasks[0].id]) {
                return <>Task is ontvangen, maar er ontbreekt informatie</>
            }
            return <QuestionnaireRenderer
                questionnaire={taskToQuestionnaireMap[subTasks[0].id]}
                inputTask={subTasks[0]}
            />
        case "rejected":
            return <div className='w-[568px]'>
                Het verzoek om {serviceName} voor {conditionName} uit te voeren is op {taskDate} afgewezen door {organizationName(task.owner)}.
            </div>
        default:
            //primary tasks cannot handle Task.stats, for example `ready`
            return <p>Task status {task?.status || "Taak niet gevonden"} is geen valide status voor een enrollment Task.</p>
    }
}
