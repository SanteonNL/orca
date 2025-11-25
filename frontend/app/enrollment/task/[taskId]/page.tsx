"use client"
import React, {useEffect, useState} from 'react'
import {useParams} from 'next/navigation'
import Loading from '@/app/enrollment/loading'
import QuestionnaireRenderer from '../../components/questionnaire-renderer'
import useEnrollment from "@/app/hooks/enrollment-hook";
import {getLaunchableApps, LaunchableApp} from "@/app/applaunch";
import {Questionnaire, ServiceRequest, Task} from "fhir/r4";
import { useClients } from '@/app/hooks/context-hook'
import PatientDetails from "@/app/enrollment/task/components/patient-details";
import TaskProgressHook from "@/app/hooks/task-progress-hook";
import TaskHeading from "@/app/enrollment/components/task-heading";
import {ChevronRight} from "lucide-react";
import TaskBody from "@/app/enrollment/components/task-body";
import Error from "@/app/error";
import {organizationNameShort} from "@/lib/fhirRender";
import {requestTitle} from "@/app/enrollment/task/components/util";
import {statusLabelLong} from "@/app/utils/mapping";

export default function EnrollmentTaskPage() {
    const {taskId} = useParams()
    const { scpClient, cpsClient } = useClients()

    const {
        task,
        subTasks,
        questionnaireMap,
        isError,
        isLoading
    } = TaskProgressHook({
        taskId: Array.isArray(taskId) ? taskId[0] : taskId!,
        cpsClient: cpsClient!,
        pollingInterval: 1000
    })
    const {patient, serviceRequest} = useEnrollment()

    const [launchableApps, setLaunchableApps] = useState<LaunchableApp[] | undefined>(undefined)
    const [currentQuestionnaire, setCurrentQuestionnaire] = useState<Questionnaire | undefined>(undefined);
    const [cpsServiceRequest, setCPSServiceRequest] = useState<ServiceRequest | undefined>(undefined);

    useEffect(() => {
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
        if (task?.status === "in-progress" && launchableApps && launchableApps.length === 1) {
            launchApp(launchableApps[0].URL)();
        }
    }, [task?.status, launchableApps])

    // Load ServiceRequest from CPS as referred to by the Task.focus, in case the context doesn't specify the ServiceRequest
    // (e.g. when not launching for a specific Task using /list)
    useEffect(() => {
        if (!cpsClient || !task || !setCPSServiceRequest) {
            return
        }
        if (!task.focus?.reference) {
            return
        }
        cpsClient.read({resourceType: 'ServiceRequest', id: task.focus.reference.replace('ServiceRequest/', '')})
            .then((sr) => {
                setCPSServiceRequest(sr as ServiceRequest)
            })
    }, [setCPSServiceRequest, cpsClient, task]);

    useEffect(() => {
        if (!questionnaireMap) {
            return undefined
        }
        if (!subTasks || subTasks.length === 0) {
            return undefined
        }
        setCurrentQuestionnaire(questionnaireMap[subTasks[0].id!!])
    }, [questionnaireMap, subTasks]);

    if (isLoading) return <Loading/>
    if (isError) {
        return <Error error={{
            name: 'TaskError',
            message: '"Er is een probleem opgetreden bij het ophalen van de taak"'
        }} reset={() => isError}/>
    }
    if (!task) {
        return <div className='w-[568px] flex flex-col gap-4'>Taak niet gevonden</div>
    }

    const lastStepTaskStates = ['accepted', 'in-progress', 'rejected', 'failed', 'completed', 'cancelled', 'on-hold']
    const isFirstStep = task.status === "requested"
    const isLastStep = lastStepTaskStates.includes(task.status)
    const breadcrumb = isFirstStep
        ? <span className='font-medium'>Controleer patiëntgegevens</span>
        : <a href={`${process.env.NEXT_PUBLIC_BASE_PATH || ""}/enrollment/new`} className="text-primary font-medium">Controleer patiëntgegevens</a>
    const serviceRequestTitle = requestTitle(serviceRequest || cpsServiceRequest)


    // Auto-launch external app when the following conditions are met:
    // - Task.status is "in-progress"
    // - There is exactly one launchable app
    // - Auto-launch is enabled
    const launchApp = (URL: string) => () => {
        window.open(URL, "_self");
    }

    const textBottom = executionTextBottom(task.status);
    return (
        <>
            <TaskHeading title={statusLabelLong(task.status, serviceRequestTitle, task.owner)}>
                <nav className={`flex items-center space-x-2 text-sm ${isLastStep ? 'invisible' : 'inherit'}`}>
                    {breadcrumb}
                    <ChevronRight className="h-4 w-4"/>
                    <span className={`first-letter:uppercase ${isFirstStep ? 'text-muted-foreground' : ''}`}>{serviceRequestTitle} instellen</span>
                </nav>
            </TaskHeading>
            <TaskBody>
                {task.status === "received" && currentQuestionnaire && subTasks?.[0] ? <>
                    <QuestionnaireRenderer
                        questionnaire={currentQuestionnaire}
                        inputTask={subTasks[0]}
                    />
                </> : (
                    <div className='w-full flex flex-col auto-cols-max gap-y-10'>
                        <div className="w-[568px] font-[500]">
                            {
                                // Either show Task.note, or a default message based on task status
                                task.note && task.note.length > 0 ? task.note.map(note => note.text).join("\n") :
                                    executionTextTop(serviceRequestTitle, task) ?? ''
                            }
                        </div>
                        <PatientDetails task={task} serviceRequest={serviceRequest || cpsServiceRequest} patient={patient}/>
                        {
                            textBottom ? <div className="w-[568px] font-[500]">{textBottom}</div> : <></>
                        }
                    </div>
                )}
            </TaskBody>
        </>
    )
}

function executionTextTop(serviceDisplay: string | undefined, task: Task) {
    switch (task.status) {
        case "requested":
        case "received":
            return `Je gaat deze patient aanmelden ${serviceDisplay ? `voor ${serviceDisplay}` : ''} ${task.owner ? ` van ${organizationNameShort(task.owner)}` : ''}.`
        case "accepted":
            return "De aanmelding is door de uitvoerende organisatie geaccepteerd, maar uitvoering is nog niet gestart."
        case "in-progress":
            return "De aanmelding is door de uitvoerende partij geaccepteerd, en uitvoering is gestart."
        case "cancelled":
            return "De aanmelding is afgebroken."
        case "rejected":
            return "De aanmelding is door de uitvoerende partij afgewezen."
        case "failed":
            return "De aanmelding is mislukt."
        case "completed":
            return "De aanmelding is door de uitvoerende partij afgerond."
        case "on-hold":
            return "De aanmelding is door de uitvoerende partij gepauzeerd."
        default:
            return null
    }
}

function executionTextBottom(taskStatus: string) {
    switch (taskStatus) {
        case "requested":
            // fallthrough
        case "received":
            return "Indien de gegevens van de patiënt niet kloppen, pas het dan aan in het EPD. Sluit daarna dit scherm en open het opnieuw om de wijzigingen te zien."
        default:
            return null
    }
}
