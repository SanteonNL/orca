import React, { useState } from 'react';
import { Patient } from 'fhir/r4';
import {
    Avatar,
    Card,
    CardContent,
    CardHeader,
    Chip,
    Divider,
    Grid,
    List,
    ListItem,
    ListItemText,
    Tab,
    Tabs,
    Typography,
    Table,
    TableBody,
    TableCell,
    TableContainer,
    TableHead,
    TableRow,
    Paper,
    Box
} from '@mui/material';
import { styled } from '@mui/material/styles';

// Styled components
const StyledCard = styled(Card)(({ theme }) => ({
    maxWidth: 800,
    margin: 'auto',
    marginTop: theme.spacing(4),
}));

const StyledAvatar = styled(Avatar)(({ theme }) => ({
    width: theme.spacing(7),
    height: theme.spacing(7),
}));

// Helper function to get patient name
const getPatientName = (patient: Patient): string => {
    const name = patient.name?.[0];
    if (!name) return 'Unknown';
    return `${name.given?.join(' ') || ''} ${name.family || ''}`.trim();
};

// Helper function to get patient initials
const getPatientInitials = (patient: Patient): string => {
    const name = patient.name?.[0];
    if (!name) return '??';
    const given = name.given?.[0] || '';
    const family = name.family || '';
    return `${given.charAt(0)}${family.charAt(0)}`.toUpperCase();
};

// DataDisplay component for rendering different types of data
const DataDisplay: React.FC<{ label: string; value: any }> = ({ label, value }) => {
    if (value === undefined || value === null) return null;

    if (typeof value === 'object' && !Array.isArray(value)) {
        return (
            <Grid container spacing={2}>
                <Grid item xs={12}>
                    <Typography variant="subtitle1">{label}</Typography>
                </Grid>
                {Object.entries(value).map(([key, val]) => (
                    <Grid item xs={12} key={key}>
                        <DataDisplay label={key} value={val} />
                    </Grid>
                ))}
            </Grid>
        );
    }

    if (Array.isArray(value)) {
        return (
            <TableContainer component={Paper}>
                <Table size="small">
                    <TableHead>
                        <TableRow>
                            <TableCell colSpan={Object.keys(value[0] || {}).length}>
                                <Typography variant="subtitle1">{label}</Typography>
                            </TableCell>
                        </TableRow>
                        <TableRow>
                            {Object.keys(value[0] || {}).map((key) => (
                                <TableCell key={key}>{key}</TableCell>
                            ))}
                        </TableRow>
                    </TableHead>
                    <TableBody>
                        {value.map((item, index) => (
                            <TableRow key={index}>
                                {Object.values(item).map((val: any, i) => (
                                    <TableCell key={i}>
                                        {typeof val === 'object' ? JSON.stringify(val) : String(val)}
                                    </TableCell>
                                ))}
                            </TableRow>
                        ))}
                    </TableBody>
                </Table>
            </TableContainer>
        );
    }

    return (
        <ListItem>
            <ListItemText
                primary={label}
                secondary={typeof value === 'boolean' ? (
                    <Chip label={value ? 'Yes' : 'No'} color={value ? 'primary' : 'default'} size="small" />
                ) : (
                    value
                )}
            />
        </ListItem>
    );
};

interface TabPanelProps {
    children?: React.ReactNode;
    index: number;
    value: number;
}

const TabPanel: React.FC<TabPanelProps> = ({ children, value, index }) => {
    return (
        <div
            role="tabpanel"
            hidden={value !== index}
            id={`patient-tabpanel-${index}`}
            aria-labelledby={`patient-tab-${index}`}
        >
            {value === index && <Box p={3}>{children}</Box>}
        </div>
    );
};

interface PatientViewProps {
    patient?: Patient;
}

export const PatientView: React.FC<PatientViewProps> = ({ patient }) => {
    const [tabValue, setTabValue] = useState(0);

    const handleTabChange = (event: React.SyntheticEvent, newValue: number) => {
        setTabValue(newValue);
    };

    if (!patient) return (
        <StyledCard>
            <CardHeader
                avatar={
                    <StyledAvatar>
                        ?
                    </StyledAvatar>
                }
                title="Patient not found"
            />
        </StyledCard>
    );

    return (
        <StyledCard>
            <CardHeader
                avatar={
                    <StyledAvatar src={patient.photo?.[0]?.url} alt={getPatientName(patient)}>
                        {getPatientInitials(patient)}
                    </StyledAvatar>
                }
                title={<Typography variant="h5">{getPatientName(patient)}</Typography>}
                subheader={`ID: ${patient.id} | DOB: ${patient.birthDate}`}
            />
            <CardContent>
                <Tabs value={tabValue} onChange={handleTabChange} aria-label="Patient information tabs">
                    <Tab label="Demographics" id="patient-tab-0" aria-controls="patient-tabpanel-0" />
                    <Tab label="Contacts" id="patient-tab-1" aria-controls="patient-tabpanel-1" />
                    <Tab label="Identifiers" id="patient-tab-2" aria-controls="patient-tabpanel-2" />
                    <Tab label="Other Information" id="patient-tab-3" aria-controls="patient-tabpanel-3" />
                </Tabs>
                <TabPanel value={tabValue} index={0}>
                    <List>
                        <DataDisplay label="Gender" value={patient.gender} />
                        <DataDisplay label="Birth Date" value={patient.birthDate} />
                        <DataDisplay label="Address" value={patient.address} />
                        <DataDisplay label="Telecom" value={patient.telecom} />
                        <DataDisplay label="Marital Status" value={patient.maritalStatus} />
                        <DataDisplay label="Multiple Birth" value={patient.multipleBirthBoolean || patient.multipleBirthInteger} />
                        <DataDisplay label="Deceased" value={patient.deceasedBoolean || patient.deceasedDateTime} />
                    </List>
                </TabPanel>
                <TabPanel value={tabValue} index={1}>
                    <DataDisplay label="Contacts" value={patient.contact} />
                </TabPanel>
                <TabPanel value={tabValue} index={2}>
                    <DataDisplay label="Identifiers" value={patient.identifier} />
                </TabPanel>
                <TabPanel value={tabValue} index={3}>
                    <List>
                        <DataDisplay label="Communication" value={patient.communication} />
                        <DataDisplay label="General Practitioner" value={patient.generalPractitioner} />
                        <DataDisplay label="Managing Organization" value={patient.managingOrganization} />
                        <DataDisplay label="Links" value={patient.link} />
                    </List>
                </TabPanel>
            </CardContent>
        </StyledCard>
    );
};

