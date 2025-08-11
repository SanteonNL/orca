"use client"
import React, {useEffect, useState} from 'react'
import useEnrollment from '@/app/hooks/enrollment-hook'
import type {Task, Patient} from "fhir/r4";
import {getPatientIdentifier} from "@/lib/fhirUtils";
import useContext from "@/app/hooks/context-hook";

export default function TaskOverviewTable() {
    const { patient } = useEnrollment()
    const { cpsClient } = useContext()
    const [tasks, setTasks] = useState([] as Task[]);

    useEffect(() => {
        if (!patient || !cpsClient) {
            return
        }
        let patientIdentifier = getPatientIdentifier(patient as Patient);
        // For testing with SMART on FHIR Synthea data (which doesn't contain a patient with BSN identifiers),
        // we fall back to the first identifier that has a system and value.
        if (!patientIdentifier) {
            console.log("Patient does not have a BSN identifier, falling back to first available identifier");
            patientIdentifier = patient?.identifier?.filter(identifier => identifier.system && identifier.value)?.[0] ?? undefined;
        }
        if (!patientIdentifier) {
            throw new Error("No patient identifier found for the patient");
        }
        cpsClient.search({
            resourceType: 'Task',
            searchParams: {
                'patient': `${patientIdentifier.system}|${patientIdentifier.value}`
            }
        }).then(bundle => {
            setTasks(bundle.entry?.map((entry: { resource: Task; }) => entry.resource as Task) ?? []);
        })
    }, [cpsClient, setTasks, patient]);

    return <div className="overflow-x-auto">
        <table className="min-w-full border border-gray-200 rounded-lg">
            <thead>
            <tr className="bg-gray-100">
                <th className="px-4 py-2 text-left">Uitvoerder</th>
                <th className="px-4 py-2 text-left">Verzoek</th>
                <th className="px-4 py-2 text-left">Status</th>
                <th className="px-4 py-2 text-left">Datum</th>
            </tr>
            </thead>
            <tbody>
            {tasks.map((task, idx) => (
                <tr key={idx} className="border-t">
                    <td className="px-4 py-2">{task.owner?.display ?? "(onbekend)"}</td>
                    <td className="px-4 py-2">{task.focus?.display}</td>
                    <td className="px-4 py-2">{task.status}</td>
                    <td className="px-4 py-2">{task.lastModified}</td>
                </tr>
            ))}
            </tbody>
        </table>
    </div>
}