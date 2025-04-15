import React from 'react';
import { Card, CardContent, Typography, List, ListItem, ListItemText } from '@mui/material';
import useBgzStore from '@/store/bgz-store';

export const DeviceUseStatementView = () => {
    const { deviceUseStatements } = useBgzStore();
    return (
        <Card>
            <CardContent>
                <Typography variant="h5" component="h2" gutterBottom>
                    Gebruik van apparaten
                </Typography>
                <List>
                    {deviceUseStatements.map((statement, index) => (
                        <ListItem key={statement.id || index}>
                            <ListItemText
                                disableTypography={true}
                                primary={`Gebruik van apparaat ${index + 1}`}
                                secondary={
                                    <>
                                        <Typography component="p" variant="body1" color="text.primary">
                                            <strong>Status:</strong> {statement.status}
                                        </Typography>
                                        <Typography component="p" variant="body1" color="text.primary">
                                            <strong>Apparaat:</strong> {statement.device.display}
                                        </Typography>
                                        <Typography component="p" variant="body1" color="text.primary">
                                            <strong>Tijdstip:</strong> {statement.timingPeriod?.start ? new Date(statement.timingPeriod.start).toLocaleDateString('nl-NL') : 'N.v.t.'}
                                        </Typography>
                                        <Typography component="p" variant="body1" color="text.primary">
                                            <strong>Locatie:</strong> {statement.bodySite?.text || 'N.v.t.'}
                                        </Typography>
                                        <Typography component="p" variant="body1" color="text.primary">
                                            <strong>Opmerkingen:</strong> {statement.note?.map(note => note.text).join(', ') || 'Geen'}
                                        </Typography>
                                        <Typography component="p" variant="body1" color="text.primary">
                                            <strong>Patiënt:</strong> {statement.subject?.display || 'Onbekend'}
                                        </Typography>
                                        <Typography component="p" variant="body1" color="text.primary">
                                            <strong>Bron:</strong> {statement.source?.display || 'Onbekend'}
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