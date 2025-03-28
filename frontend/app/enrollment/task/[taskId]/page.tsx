"use client"
import React, { useEffect, useState } from 'react'
import useTaskProgressStore from '@/lib/store/task-progress-store'
import { useParams } from 'next/navigation'
import Loading from '@/app/enrollment/loading'
import QuestionnaireRenderer from '../../components/questionnaire-renderer'
import useEnrollmentStore from "@/lib/store/enrollment-store";
import { patientName, organizationName } from "@/lib/fhirRender";
import DataViewer from '@/components/data-viewer'
import { viewerFeatureIsEnabled } from '@/app/actions'
import TaskSseConnectionStatus from '../../components/sse-connection-status'

export default function EnrollmentTaskPage() {
    const { taskId } = useParams()
    const { task, loading, initialized, setSelectedTaskId, subTasks, taskToQuestionnaireMap } = useTaskProgressStore()
    const { patient } = useEnrollmentStore()
    const [viewerFeatureEnabled, setViewerFeatureEnabled] = useState(false)
    const [showViewer, setShowViewer] = useState(false)

    useEffect(() => {
        if (taskId) {
            //TODO: Currently we only have one Questionnaire per enrollment flow. But we support multiple. The UX for multiple still needs to be made. When it's there, this is the place to add it
            const selectedTaskId = Array.isArray(taskId) ? taskId[0] : taskId;
            setSelectedTaskId(selectedTaskId);
        }
    }, [taskId, setSelectedTaskId])

    useEffect(() => {
        setShowViewer(viewerFeatureEnabled && !!task && (task.status === "accepted" || task.status === "in-progress"))
    }, [task, viewerFeatureEnabled])

    useEffect(() => {
        viewerFeatureIsEnabled()
            .then((enabled) => {
                setViewerFeatureEnabled(enabled)
            })
    }, [])

    if (loading || !initialized) return <Loading />

    if (!task) {
        return <div className='w-[568px] flex flex-col gap-4'>Taak niet gevonden</div>
    }

    const StatusElement = ({ label, value, noUpperCase }: { label: string, value: string, noUpperCase?: boolean | undefined }) =>
        <>
            <div>{label}:</div>
            <div className={"font-[500] " + !noUpperCase ? "first-letter:uppercase" : ""}>{value}</div>
        </>

    if (task.status === "received") {
        if (!taskToQuestionnaireMap || !subTasks?.[0]?.id || !taskToQuestionnaireMap[subTasks[0].id]) {
            return <>Task is ontvangen, maar er ontbreekt informatie.</>
        }
        return <>
            <QuestionnaireRenderer
                questionnaire={taskToQuestionnaireMap[subTasks[0].id]}
                inputTask={subTasks[0]}
            />
            <TaskSseConnectionStatus />
        </>
    } else {
        return <div className='w-full flex flex-col auto-cols-max'>
            {
                task && executionText(task.status) ?
                    <p className="w-[568px] text-muted-foreground pb-8">{executionText(task.status)}</p> : <></>
            }
            <div className="w-[568px] grid grid-cols-[1fr_2fr] gap-y-4">
                <StatusElement label="Patiënt" value={patient ? patientName(patient) : "Onbekend"} noUpperCase={true} />
                <StatusElement label="Verzoek" value={task?.focus?.display || "Onbekend"} />
                <StatusElement label="Diagnose" value={task?.reasonCode?.coding?.[0].display || "Onbekend"} />
                <StatusElement label="Uitvoerende organisatie" value={organizationName(task.owner)} />
                <StatusElement label="Status"
                    value={statusLabel(task.status) + " op " + (task?.meta?.lastUpdated ? new Date(task.meta.lastUpdated).toLocaleDateString("nl-NL") : "Onbekend")} />
                {task.statusReason
                    ? <StatusElement label="Statusreden"
                        value={task.statusReason.text ?? task.statusReason.coding?.at(0)?.code ?? "Onbekend"} />
                    : <></>
                }
            </div>
            {showViewer && <DataViewer task={task} />}
            <TaskSseConnectionStatus />
        </div>
    }
}

function statusLabel(taskStatus: string): string {
    switch (taskStatus) {
        case "accepted":
            return "Geaccepteerd"
        case "completed":
            return "Afgerond"
        case "cancelled":
            return "Geannuleerd"
        case "failed":
            return "Mislukt"
        case "in-progress":
            return "In behandeling"
        case "on-hold":
            return "Gepauzeerd"
        case "requested":
            return "Verstuurd"
        case "received":
            return "Ontvangen"
        case "rejected":
            return "Afgewezen"
        default:
            return taskStatus
    }
}

function executionText(taskStatus: string) {
    switch (taskStatus) {
        case "requested":
            return "Het verzoek is door de uitvoerende organisatie ontvangen, maar nog niet beoordeeld."
        case "accepted":
            return "Het verzoek is door de uitvoerende organisatie geaccepteerd, maar uitvoering is nog niet gestart."
        case "in-progress":
            return "Het verzoek is door de uitvoerende partij geaccepteerd, en uitvoering is gestart."
        case "cancelled":
            return "Het verzoek is afgebroken."
        case "rejected":
            return "Het verzoek is door de uitvoerende partij afgewezen."
        case "failed":
            return "Het verzoek is mislukt."
        case "completed":
            return "Het verzoek is door de uitvoerende partij afgerond."
        case "on-hold":
            return "Het verzoek is door de uitvoerende partij gepauzeerd."
        default:
            return null
    }
}
