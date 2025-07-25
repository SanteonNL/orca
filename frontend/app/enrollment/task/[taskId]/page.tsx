"use client"
import React, { useEffect, useState } from 'react'
import useTaskProgressStore from '@/lib/store/task-progress-store'
import { useParams } from 'next/navigation'
import Loading from '@/app/enrollment/loading'
import QuestionnaireRenderer from '../../components/questionnaire-renderer'
import useEnrollmentStore from "@/lib/store/enrollment-store";
import {patientName, organizationName} from "@/lib/fhirRender";
import {getLaunchableApps, LaunchableApp} from "@/app/applaunch";
import {Questionnaire} from "fhir/r4";
import {Button, ThemeProvider} from "@mui/material";
import {defaultTheme} from "@/app/theme";

export default function EnrollmentTaskPage() {
    const { taskId } = useParams()
    const { task, loading, initialized, setSelectedTaskId, subTasks, taskToQuestionnaireMap } = useTaskProgressStore()
    const { patient, serviceRequest } = useEnrollmentStore()
    const [launchableApps, setLaunchableApps] = useState<LaunchableApp[] | undefined>(undefined)
    const [currentQuestionnaire, setCurrentQuestionnaire] = useState<Questionnaire | undefined>(undefined);
    useEffect(() => {
        if (taskId) {
            console.log(`Task ID from URL: ${taskId}`);
            //TODO: Currently we only have one Questionnaire per enrollment flow. But we support multiple. The UX for multiple still needs to be made. When it's there, this is the place to add it
            const selectedTaskId = Array.isArray(taskId) ? taskId[0] : taskId;
            setSelectedTaskId(selectedTaskId);
        }
    }, [taskId, setSelectedTaskId])

    useEffect(()=>{
        const primaryTaskPerformer = serviceRequest?.performer?.[0].identifier;
        if (!primaryTaskPerformer) {
            return
        }
        getLaunchableApps(primaryTaskPerformer)
            .then((apps) => {
                setLaunchableApps(apps)
            })
    }, [serviceRequest, setLaunchableApps])

    useEffect(() => {
        if (!taskToQuestionnaireMap) {
            return undefined
        }
        if (!subTasks || subTasks.length === 0) {
            return undefined
        }
        setCurrentQuestionnaire(taskToQuestionnaireMap[subTasks[0].id!!])
    }, [taskToQuestionnaireMap, subTasks]);

    if (loading || !initialized) return <Loading />

    if (!task) {
        return <div className='w-[568px] flex flex-col gap-4'>Taak niet gevonden</div>
    }

    const StatusElement = ({ label, value, noUpperCase }: { label: string, value: string, noUpperCase?: boolean | undefined }) =>
        <>
            <div className={"font-[500]"}>{label}:</div>
            <div className={!noUpperCase ? "first-letter:uppercase" : ""}>{value}</div>
        </>

    // Auto-launch external app when the following conditions are met:
    // - Task.status is "in-progress"
    // - There is exactly one launchable app
    // - Auto-launch is enabled
    const autoLaunchExternalApps = process.env.NEXT_PUBLIC_AUTOLAUNCH_EXTERNAL_APP;
    const launchApp = (URL: string) => () => {
        window.open(URL, "_self");
    }
    if (task.status === "in-progress" && launchableApps && launchableApps.length === 1 && autoLaunchExternalApps) {
        launchApp(launchableApps[0].URL)();
    }

    if (task.status === "received" && currentQuestionnaire && subTasks?.[0]) {
        return <>
            <QuestionnaireRenderer
                questionnaire={currentQuestionnaire}
                inputTask={subTasks[0]}
            />
        </>
    } else {
        return <div className='w-full flex flex-col auto-cols-max gap-y-10'>
            <div className="w-[568px] font-[500]">
            {
                // Either show Task.note, or a default message based on task status
                task.note && task.note.length > 0 ? task.note.map(note => note.text).join("\n") :
                executionText(task.status) ? executionText(task.status) : ''
            }
            </div>
            <div className="w-[568px] grid grid-cols-[1fr_2fr] gap-y-4">
                <StatusElement label="Patiënt" value={patient ? patientName(patient) : "Onbekend"} noUpperCase={true} />
                <StatusElement label="E-mailadres" value={patient?.telecom?.find(m => m.system === 'email')?.value ?? 'Onbekend'} />
                <StatusElement label="Telefoonnummer" value={patient?.telecom?.find(m => m.system === 'phone')?.value ?? 'Onbekend'} />
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

            {task.status === "in-progress" && !autoLaunchExternalApps && launchableApps && launchableApps.length > 0 &&
                <div className="w-[568px]">
                    <ThemeProvider theme={defaultTheme}>
                        {launchableApps.map((app, index) => (
                            <Button
                                key={index}
                                variant="contained"
                                className="mb-2"
                                onClick={launchApp(app.URL)}
                            >{app.Name}</Button>
                        ))}
                    </ThemeProvider>
                </div>
            }
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
