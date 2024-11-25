import { Grid } from '@mui/material'
import React from 'react'
import { PatientView } from './PatientView'
import { AllergyIntoleranceView } from './AllergyIntoleranceView'
import { ConditionView } from './ConditionView'
import { MedicationStatementView } from './MedicationStatementView'
import { ImmunizationView } from './ImmunizationView'
import { ProcedureView } from './ProcedureView'
import { CoverageView } from './CoverageView'
import { ConsentView } from './ConsentView'
import { ObservationView } from './ObservationView'
import useBgzStore from '@/store/bgz-store'

export default function BgzRecordsViewer() {

    const { patient, allergyIntolerances, conditions, consents, coverages, immunizations, medicationStatements, observations, procedures } = useBgzStore()

    return (
        <Grid spacing={3}>
            <Grid item xs={12}>
                <PatientView patient={patient} />
            </Grid>
            {allergyIntolerances.map((allergyIntolerance) => (
                <Grid item xs={12} md={6} key={allergyIntolerance.id}>
                    <AllergyIntoleranceView allergyIntolerance={allergyIntolerance} />
                </Grid>
            ))}
            {conditions.map((condition) => (
                <Grid item xs={12} md={6} key={condition.id}>
                    <ConditionView condition={condition} />
                </Grid>
            ))}
            {medicationStatements.map((medicationStatement) => (
                <Grid item xs={12} md={6} key={medicationStatement.id}>
                    <MedicationStatementView medicationStatement={medicationStatement} />
                </Grid>
            ))}
            {immunizations.map((immunization) => (
                <Grid item xs={12} md={6} key={immunization.id}>
                    <ImmunizationView immunization={immunization} />
                </Grid>
            ))}
            {procedures.map((procedure) => (
                <Grid item xs={12} md={6} key={procedure.id}>
                    <ProcedureView procedure={procedure} />
                </Grid>
            ))}
            {coverages.map((coverage) => (
                <Grid item xs={12} md={6} key={coverage.id}>
                    <CoverageView coverage={coverage} />
                </Grid>
            ))}
            {consents.map((consent) => (
                <Grid item xs={12} md={6} key={consent.id}>
                    <ConsentView consent={consent} />
                </Grid>
            ))}
            {observations.map((observation) => (
                <Grid item xs={12} md={6} key={observation.id}>
                    <ObservationView observation={observation} />
                </Grid>
            ))}
        </Grid>
    )
}
