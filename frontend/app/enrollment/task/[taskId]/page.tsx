"use client"
import React, { useEffect, useState} from 'react'
import { useParams } from 'next/navigation'
import Loading from '@/app/enrollment/loading'
import QuestionnaireRenderer from '../../components/questionnaire-renderer'
import useEnrollmentStore from "@/lib/store/enrollment-store";
import {getLaunchableApps, LaunchableApp} from "@/app/applaunch";
import {Questionnaire} from "fhir/r4";
import {Button, ThemeProvider} from "@mui/material";
import {defaultTheme} from "@/app/theme";
import {useContextStore} from "@/lib/store/context-store";
import PatientDetails from "@/app/enrollment/task/components/patient-details";
import TaskProgressHook from "@/app/hooks/task-progress-hook";

export default function EnrollmentTaskPage() {
    const { taskId } = useParams()
    const { scpClient, cpsClient } = useContextStore()
    const { task, subTasks, questionnaireMap, isError, isLoading } = TaskProgressHook({ taskId: Array.isArray(taskId) ? taskId[0] : taskId!, cpsClient: cpsClient!, pollingInterval: 1000 })
    const { patient, serviceRequest } = useEnrollmentStore()

    const [launchableApps, setLaunchableApps] = useState<LaunchableApp[] | undefined>(undefined)
    const [currentQuestionnaire, setCurrentQuestionnaire] = useState<Questionnaire | undefined>(undefined);

    useEffect(()=>{
        const primaryTaskPerformer = serviceRequest?.performer?.[0].identifier;
        if (!primaryTaskPerformer || !scpClient) {
            return
        }
        getLaunchableApps(scpClient, primaryTaskPerformer)
            .then((apps) => {
                setLaunchableApps(apps)
            })
    }, [serviceRequest, setLaunchableApps, scpClient])

    useEffect(() => {
        if (!questionnaireMap) {
            return undefined
        }
        if (!subTasks || subTasks.length === 0) {
            return undefined
        }
        setCurrentQuestionnaire(questionnaireMap[subTasks[0].id!!])
    }, [questionnaireMap, subTasks]);

    if (isLoading) return <Loading />

    if (!task) {
        return <div className='w-[568px] flex flex-col gap-4'>Taak niet gevonden</div>
    }



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
            <PatientDetails task={task} patient={patient} />

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
