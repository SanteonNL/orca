import React from 'react';
import { Card, CardContent, Typography, List, ListItem, ListItemText } from '@mui/material';
import useBgzStore from '@/store/bgz-store';

export const AppointmentView = () => {
    const { appointments } = useBgzStore();

    return (
        <List>
            {appointments.map((appointment, index) => (
                <Card key={appointment.id || index} style={{ marginBottom: '1rem' }}>
                    <CardContent>
                        <Typography variant="h5" component="h2" gutterBottom>
                            Appointment {index + 1}
                        </Typography>
                        <Typography variant="body1" component="p" color="text.primary">
                            <strong>Status:</strong> {appointment.status}
                        </Typography>
                        <Typography variant="body1" component="p" color="text.primary">
                            <strong>Start:</strong> {new Date(appointment.start || '').toLocaleDateString('nl-NL')}
                        </Typography>
                        <Typography variant="body1" component="p" color="text.primary">
                            <strong>End:</strong> {new Date(appointment.end || '').toLocaleDateString('nl-NL')}
                        </Typography>
                        <Typography variant="body1" component="p" color="text.primary">
                            <strong>Type:</strong> {appointment.appointmentType?.text}
                        </Typography>
                        <Typography variant="body1" component="p" color="text.primary">
                            <strong>Identifier:</strong> {appointment.identifier?.[0]?.value}
                        </Typography>
                        <Typography variant="body1" component="p" color="text.primary">
                            <strong>Reason:</strong> {appointment.reason?.[0]?.text}
                        </Typography>
                        <Typography variant="body1" component="p" color="text.primary">
                            <strong>Participants:</strong>
                        </Typography>
                        <List>
                            {appointment.participant?.map((participant, pIndex) => (
                                <ListItem key={pIndex}>
                                    <ListItemText
                                        primary={participant.actor?.display}
                                        secondary={`Status: ${participant.status}`}
                                    />
                                </ListItem>
                            ))}
                        </List>
                        <Typography variant="body1" component="p" color="text.primary">
                            <strong>Requested Period:</strong> {new Date(appointment.requestedPeriod?.[0]?.start || '').toLocaleDateString('nl-NL')} - {new Date(appointment.requestedPeriod?.[0]?.end || '').toLocaleDateString('nl-NL')}
                        </Typography>
                        <Typography variant="body1" component="p" color="text.primary">
                            <strong>Opmerking:</strong> {appointment.comment?.[0] || 'Geen opmerkingen'}
                        </Typography>
                    </CardContent>
                </Card>
            ))}
        </List>
    );
};
