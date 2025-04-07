import React from 'react'
import { Box, Tabs, Tab } from '@mui/material'
import { PatientView } from './PatientView'
import { AppointmentView } from './AppointmentView'
import { ImmunizationView } from './ImmunizationView'
import { NutritionOrderView } from './NutritionOrderView'
import { ImmunizationRecommendationView } from './ImmunizationRecommendationView'
import { DeviceRequestView } from './DeviceRequestView'
import { ObservationView } from './ObservationView'
import { FlagView } from './FlagView'
import { ProcedureView } from './ProcedureView'
import { ConditionView } from './ConditionView'
import { ConsentView } from './ConsentView'
import { EncounterView } from './EncounterView'
import { ServiceRequestView } from './ServiceRequestView'
import { DeviceUseStatementView } from './DeviceUseStatementView'
import { MedicationRequestView } from './MedicationRequestView'
import { CoverageView } from './CoverageView'

interface TabPanelProps {
    children?: React.ReactNode;
    index: number;
    value: number;
}

function CustomTabPanel(props: TabPanelProps) {
    const { children, value, index, ...other } = props;

    return (
        <div
            role="tabpanel"
            hidden={value !== index}
            id={`simple-tabpanel-${index}`}
            aria-labelledby={`simple-tab-${index}`}
            {...other}
        >
            {value === index && <Box sx={{ p: 3 }}>{children}</Box>}
        </div>
    );
}

function a11yProps(index: number) {
    return {
        id: `simple-tab-${index}`,
        'aria-controls': `simple-tabpanel-${index}`,
    };
}

export default function BgzRecordsViewer() {
    const [value, setValue] = React.useState(0);

    const handleChange = (event: React.SyntheticEvent, newValue: number) => {
        setValue(newValue);
    };

    return (
        <Box sx={{ width: '100%' }}>
            <Box sx={{ borderBottom: 1, borderColor: 'divider' }}>
                <Tabs value={value} onChange={handleChange} variant="scrollable" scrollButtons="auto">
                    <Tab label="Patient" {...a11yProps(0)} />
                    <Tab label="Appointments" {...a11yProps(1)} />
                    <Tab label="Immunizations" {...a11yProps(2)} />
                    <Tab label="Nutrition Orders" {...a11yProps(3)} />
                    <Tab label="Immunization Recommendations" {...a11yProps(4)} />
                    <Tab label="Device Requests" {...a11yProps(5)} />
                    <Tab label="Observations" {...a11yProps(6)} />
                    <Tab label="Flags" {...a11yProps(7)} />
                    <Tab label="Procedures" {...a11yProps(8)} />
                    <Tab label="Conditions" {...a11yProps(9)} />
                    <Tab label="Consents" {...a11yProps(10)} />
                    <Tab label="Encounters" {...a11yProps(11)} />
                    <Tab label="Service Requests" {...a11yProps(12)} />
                    <Tab label="Device Use Statements" {...a11yProps(13)} />
                    <Tab label="Medication Requests" {...a11yProps(14)} />
                    <Tab label="Coverages" {...a11yProps(15)} />
                </Tabs>
            </Box>
            <CustomTabPanel value={value} index={0}>
                <PatientView />
            </CustomTabPanel>
            <CustomTabPanel value={value} index={1}>
                <AppointmentView />
            </CustomTabPanel>
            <CustomTabPanel value={value} index={2}>
                <ImmunizationView />
            </CustomTabPanel>
            <CustomTabPanel value={value} index={3}>
                <NutritionOrderView />
            </CustomTabPanel>
            <CustomTabPanel value={value} index={4}>
                <ImmunizationRecommendationView />
            </CustomTabPanel>
            <CustomTabPanel value={value} index={5}>
                <DeviceRequestView />
            </CustomTabPanel>
            <CustomTabPanel value={value} index={6}>
                <ObservationView />
            </CustomTabPanel>
            <CustomTabPanel value={value} index={7}>
                <FlagView />
            </CustomTabPanel>
            <CustomTabPanel value={value} index={8}>
                <ProcedureView />
            </CustomTabPanel>
            <CustomTabPanel value={value} index={9}>
                <ConditionView />
            </CustomTabPanel>
            <CustomTabPanel value={value} index={10}>
                <ConsentView />
            </CustomTabPanel>
            <CustomTabPanel value={value} index={11}>
                <EncounterView />
            </CustomTabPanel>
            <CustomTabPanel value={value} index={12}>
                <ServiceRequestView />
            </CustomTabPanel>
            <CustomTabPanel value={value} index={13}>
                <DeviceUseStatementView />
            </CustomTabPanel>
            <CustomTabPanel value={value} index={14}>
                <MedicationRequestView />
            </CustomTabPanel>
            <CustomTabPanel value={value} index={15}>
                <CoverageView />
            </CustomTabPanel>
        </Box>
    );
}