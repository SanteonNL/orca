import React, { useState } from 'react';
import { Address } from 'fhir/r4';
import {
    Avatar,
    Box,
    Card,
    Tab,
    Tabs,
    Typography,
    List,
    ListItem,
    ListItemText,
    Paper,
    useTheme,
    styled,
} from '@mui/material';
import useBgzStore from '@/store/bgz-store';

const StyledCard = styled(Card)(({ theme }) => ({
    backgroundColor: theme.palette.background.paper,
    color: theme.palette.text.primary,
}));

const StyledHeader = styled(Box)(({ theme }) => ({
    padding: theme.spacing(3),
    display: 'flex',
    alignItems: 'center',
    gap: theme.spacing(2),
}));

const StyledAvatar = styled(Avatar)(({ theme }) => ({
    width: theme.spacing(6),
    height: theme.spacing(6),
    backgroundColor: theme.palette.grey[600],
}));

const StyledTabs = styled(Tabs)(({ theme }) => ({
    borderBottom: `1px solid ${theme.palette.divider}`,
    '& .MuiTab-root': {
        color: theme.palette.text.secondary,
        '&.Mui-selected': {
            color: theme.palette.primary.main,
        },
    },
}));

interface TabPanelProps {
    children?: React.ReactNode;
    index: number;
    value: number;
}

const TabPanel = (props: TabPanelProps) => {
    const { children, value, index, ...other } = props;

    return (
        <div
            role="tabpanel"
            hidden={value !== index}
            id={`patient-tabpanel-${index}`}
            aria-labelledby={`patient-tab-${index}`}
            {...other}
        >
            {value === index && <Box sx={{ p: 3 }}>{children}</Box>}
        </div>
    );
};

const DataItem = ({
    label,
    value,
}: {
    label: string;
    value: string | undefined;
}) => (
    <ListItem>
        <ListItemText
            primary={label}
            secondary={value || 'N/A'}
            primaryTypographyProps={{
                variant: 'subtitle2',
                color: 'text.secondary',
            }}
            secondaryTypographyProps={{
                variant: 'body1',
                color: 'text.primary',
            }}
        />
    </ListItem>
);

export const PatientView = () => {
    const { patient } = useBgzStore();
    const [tabValue, setTabValue] = useState(0);
    const theme = useTheme();

    if (!patient) return <div>Patient niet gevonden</div>;

    const handleTabChange = (event: React.SyntheticEvent, newValue: number) => {
        setTabValue(newValue);
    };

    const getInitials = () => {
        const name = patient.name?.[0];
        if (!name) return '??';
        const given = name.given?.[0] || '';
        return given.substring(0, 2).toUpperCase();
    };

    const getPatientName = () => {
        const name = patient.name?.[0];
        if (!name) return 'Unknown';
        return `${name.given?.join(' ') || ''} ${name.family || ''}`.trim();
    };

    const getFormattedAddress = (address: Address) => {
        const parts = [];
        if (address.line) parts.push(...address.line);
        if (address.city) parts.push(address.city);
        if (address.postalCode) parts.push(address.postalCode);
        if (address.country) parts.push(address.country);
        return parts.join(', ');
    };

    return (
        <StyledCard elevation={0}>
            <StyledHeader>
                <StyledAvatar>{getInitials()}</StyledAvatar>
                <Box>
                    <Typography variant="h6" component="h2">
                        {getPatientName()}
                    </Typography>
                    <Typography variant="body2" color="text.secondary">
                        ID: {patient.id} | DOB: {patient.birthDate}
                    </Typography>
                </Box>
            </StyledHeader>

            <StyledTabs
                value={tabValue}
                onChange={handleTabChange}
                aria-label="Patient information tabs"
            >
                <Tab label="Demographics" />
                <Tab label="Contacts" />
                <Tab label="Identifiers" />
                <Tab label="Other Information" />
            </StyledTabs>

            <TabPanel value={tabValue} index={0}>
                <List>
                    <DataItem label="Gender" value={patient.gender} />
                    <DataItem label="Birth Date" value={patient.birthDate} />
                    {patient.address?.map((address, index) => (
                        <DataItem
                            key={index}
                            label="Address"
                            value={getFormattedAddress(address)}
                        />
                    ))}
                    {patient.telecom?.map((telecom, index) => (
                        <DataItem
                            key={index}
                            label={`${telecom.system} (${telecom.use || 'primary'})`}
                            value={telecom.value}
                        />
                    ))}
                    <DataItem
                        label="Marital Status"
                        value={
                            patient.maritalStatus?.text ||
                            patient.maritalStatus?.coding?.[0]?.display
                        }
                    />
                </List>
            </TabPanel>

            <TabPanel value={tabValue} index={1}>
                <List>
                    {patient.contact?.map((contact, index) => (
                        <React.Fragment key={index}>
                            <DataItem
                                label="Contact Name"
                                value={
                                    contact.name
                                        ? `${contact.name.given?.join(' ')} ${contact.name.family}`
                                        : undefined
                                }
                            />
                            <DataItem
                                label="Relationship"
                                value={
                                    contact.relationship?.[0]?.text ||
                                    contact.relationship?.[0]?.coding?.[0]
                                        ?.display
                                }
                            />
                            {contact.telecom?.map((telecom, telecomIndex) => (
                                <DataItem
                                    key={telecomIndex}
                                    label={`Contact ${telecom.system} (${telecom.use || 'primary'})`}
                                    value={telecom.value}
                                />
                            ))}
                        </React.Fragment>
                    ))}
                </List>
            </TabPanel>

            <TabPanel value={tabValue} index={2}>
                <List>
                    {patient.identifier?.map((identifier, index) => (
                        <DataItem
                            key={index}
                            label={identifier.system || 'Identifier'}
                            value={identifier.value}
                        />
                    ))}
                </List>
            </TabPanel>

            <TabPanel value={tabValue} index={3}>
                <List>
                    {patient.generalPractitioner?.map((gp, index) => (
                        <DataItem
                            key={index}
                            label="General Practitioner"
                            value={gp.display}
                        />
                    ))}
                    <DataItem
                        label="Multiple Birth"
                        value={
                            patient.multipleBirthBoolean === true ? 'Yes' : 'No'
                        }
                    />
                    <DataItem
                        label="Deceased"
                        value={patient.deceasedBoolean === true ? 'Yes' : 'No'}
                    />
                </List>
            </TabPanel>
        </StyledCard>
    );
};
