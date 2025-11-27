"use client"
import React, {useEffect, useState} from 'react'
import useEnrollment from '@/app/hooks/enrollment-hook'
import type {Task, Patient, Bundle, Identifier} from "fhir/r4";
import {getPatientIdentifier, identifierToToken} from "@/lib/fhirUtils";
import { useClients } from '@/app/hooks/context-hook';
import {taskStatusLabel} from "@/app/utils/mapping";
import {useRouter} from "next/navigation";

export default function TaskOverviewTable() {
    const {patient} = useEnrollment()
    const { cpsClient } = useClients()
    const [tasks, setTasks] = useState([] as Task[]);

    useEffect(() => {
        if (!patient || !cpsClient) {
            return
        }
        // List all Tasks for the patient. For that, we need the Patient FHIR resource from the CPS,
        // because searching for FHIR Tasks requires a Patient reference (it doesn't work with identifiers).
        // So, we first need to look up the patient resource in the CPS.
        let patientIdentifier = getPatientIdentifier(patient as Patient);
        // For testing with SMART on FHIR Synthea data (which doesn't contain a patient with BSN identifiers),
        // we fall back to the first identifier that has a system and value.
        if (!patientIdentifier) {
            console.log("Patient does not have a BSN identifier, falling back to first available identifier");
            patientIdentifier = patient?.identifier?.filter(identifier => identifier.system && identifier.value)?.[0] ?? undefined;
        }
        if (!patientIdentifier) {
            throw new Error("No identifier found for the patient");
        }
        fetchTasksForPatient(patientIdentifier, cpsClient).then((tasks) => {
            setTasks(tasks);
        })
    }, [cpsClient, setTasks, patient]);

    const router = useRouter()
    const openTask = (task: Task) => {
        router.push(`/enrollment/task/${task.id}`)
    };
    useEffect(() => {
        if (router && tasks && tasks.length === 1) {
            console.log('Found 1 Task, redirecting to it');
            openTask(tasks[0])
        }
    }, [tasks, router]);

    return <div className="overflow-x-auto">
        <table className="min-w-full border border-gray-200 rounded-lg">
            <thead>
            <tr className="bg-gray-100">
                <th className="px-4 py-2 text-left">Datum</th>
                <th className="px-4 py-2 text-left">Type</th>
                <th className="px-4 py-2 text-left">Status</th>
                <th className="px-4 py-2 text-left">Uitvoerder</th>
            </tr>
            </thead>
            <tbody>
            {tasks.map((task, idx) => (
                <tr key={idx} className="border-t" onClick={() => openTask(task)} style={{cursor: 'pointer'}}>
                    <td className="px-4 py-2">{new Date(task.meta!.lastUpdated!).toLocaleString("nl-NL")}</td>
                    <td className="px-4 py-2">{task.focus?.display}</td>
                    <td className="px-4 py-2">{taskStatusLabel(task.status)}</td>
                    <td className="px-4 py-2">{task.owner?.display ?? "(onbekend)"}</td>
                </tr>
            ))}
            </tbody>
        </table>
    </div>
}

async function fetchTasksForPatient(patientId: Identifier, cpsClient: any): Promise<Task[]> {
    const patientBundle = await cpsClient.search({
        resourceType: 'Patient',
        searchParams: {
            'identifier': identifierToToken(patientId) || ''
        },
        options: {postSearch: true},
        headers: {
            'Content-Type': 'application/x-www-form-urlencoded'
        }
    });
    const patients = (patientBundle as Bundle).entry?.map((entry => entry.resource as Patient)) ?? [];

    const taskBundle = await cpsClient.search({
        resourceType: 'Task',
        searchParams: {
            'patient': patients.map(p => `Patient/${p.id}`).join(","),
            '_sort': '-_lastUpdated',
        },
        options: {postSearch: true},
        headers: {
            'Content-Type': 'application/x-www-form-urlencoded'
        }
    });
    const allTasks = (taskBundle as Bundle).entry?.map((entry => entry.resource as Task)) ?? [];
    return allTasks?.filter(task => !task.partOf)
}