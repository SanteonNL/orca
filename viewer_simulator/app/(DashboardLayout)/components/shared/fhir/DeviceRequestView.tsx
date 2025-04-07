import React from 'react';
import { Card, CardContent, Typography, List, ListItem, ListItemText } from '@mui/material';
import useBgzStore from '@/store/bgz-store';

export const DeviceRequestView = () => {

    const { deviceRequests } = useBgzStore();
    return (
        <Card>
            <CardContent>
                <Typography variant="h5" component="h2" gutterBottom>
                    Device Requests
                </Typography>
                <List>
                    {deviceRequests.map((request, index) => (
                        <ListItem key={request.id || index}>
                            <ListItemText
                                primary={`Verzoek voor apparaat ${index + 1}`}
                                secondary={
                                    <>
                                        <Typography component="p" variant="body2" color="text.primary">
                                            <strong>Status:</strong> {request.status} ({request._status?.extension?.[0]?.valueCodeableConcept?.text})
                                        </Typography>
                                        <Typography component="p" variant="body1" color="text.primary">
                                            <strong>Intentie:</strong> {request.intent}
                                        </Typography>
                                        <Typography component="p" variant="body1" color="text.primary">
                                            <strong>Prioriteit:</strong> {request.priority}
                                        </Typography>
                                        <Typography component="p" variant="body1" color="text.primary">
                                            <strong>Code:</strong> {request.codeReference?.display || request.codeCodeableConcept?.text}
                                        </Typography>
                                        <Typography component="p" variant="body1" color="text.primary">
                                            <strong>Patiënt:</strong> {request.subject?.display}
                                        </Typography>
                                        <Typography component="p" variant="body1" color="text.primary">
                                            <strong>Uitvoerende:</strong> {request.performer?.display}
                                        </Typography>
                                        <Typography component="p" variant="body1" color="text.primary">
                                            <strong>Startdatum:</strong> {request.occurrencePeriod?.start ? new Date(request.occurrencePeriod.start).toLocaleDateString('nl-NL') : 'N/A'}
                                        </Typography>
                                    </>
                                }
                            />
                        </ListItem>
                    ))}
                </List>
            </CardContent>
        </Card>
    );
};

