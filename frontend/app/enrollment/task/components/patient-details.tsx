import React, {useEffect} from 'react';
import { Task, Patient } from 'fhir/r4';
import { patientName, organizationName } from '@/lib/fhirRender';
import StatusElement from './status-element';
import {taskStatusLabel} from "@/app/utils/mapping";

interface PatientDetailsProps {
  task: Task;
  patient: Patient | undefined;
}

export default function PatientDetails({ task, patient }: PatientDetailsProps) {
  return (
    <div className="w-[568px] grid grid-cols-[1fr_2fr] gap-y-4">
      <StatusElement label="Patiënt" value={patient ? patientName(patient) : "Onbekend"} noUpperCase={true} />
      <StatusElement label="E-mailadres" value={patient?.telecom?.find(m => m.system === 'email')?.value ?? 'Onbekend'} />
      <StatusElement label="Telefoonnummer" value={patient?.telecom?.find(m => m.system === 'phone')?.value ?? 'Onbekend'} />
      <StatusElement label="Verzoek" value={task?.focus?.display || "Onbekend"} />
      <StatusElement label="Diagnose" value={task?.reasonCode?.coding?.[0].display || "Onbekend"} />
      <StatusElement label="Uitvoerende organisatie" value={organizationName(task.owner)} />
      <StatusElement label="Status"
        value={taskStatusLabel(task.status) + " op " + (task?.meta?.lastUpdated ? new Date(task.meta.lastUpdated).toLocaleDateString("nl-NL") : "Onbekend")} />
      {task.statusReason
        ? <StatusElement label="Statusreden"
            value={task.statusReason.text ?? task.statusReason.coding?.at(0)?.code ?? "Onbekend"} />
        : <></>
      }
    </div>
  );
}
