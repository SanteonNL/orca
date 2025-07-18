import React from 'react';
import { Card, CardContent, Typography, List, ListItem, ListItemText } from '@mui/material';
import useBgzStore from '@/store/bgz-store';

export const EncounterView = () => {
    const { encounters } = useBgzStore();
    return (
        <Card>
            <CardContent>
                <Typography variant="h5" component="h2" gutterBottom>
                    Ontmoetingen
                </Typography>
                <List>
                    {encounters.map((encounter, index) => (
                        <ListItem key={encounter.id || index}>
                            <ListItemText
                                primary={`Ontmoeting ${index + 1}`}
                                secondary={
                                    <>
                                        <Typography component="p" variant="body1" color="text.primary">
                                            <strong>Status:</strong> {encounter.status}
                                        </Typography>
                                        <Typography component="p" variant="body1" color="text.primary">
                                            <strong>Klasse:</strong> {encounter.class?.display}
                                        </Typography>
                                        <Typography component="p" variant="body1" color="text.primary">
                                            <strong>Type:</strong> {encounter.type?.[0]?.text || 'N/A'}
                                        </Typography>
                                        <Typography component="p" variant="body1" color="text.primary">
                                            <strong>Periode:</strong> {encounter.period?.start ? new Date(encounter.period?.start).toLocaleDateString('nl-NL') : 'N/A'} - {encounter.period?.end ? new Date(encounter.period?.end).toLocaleDateString('nl-NL') : 'Lopend'}
                                        </Typography>
                                        <Typography component="p" variant="body1" color="text.primary">
                                            <strong>Reden:</strong> {encounter.reason?.[0]?.text || 'N/A'}
                                        </Typography>
                                        <Typography component="p" variant="body1" color="text.primary">
                                            <strong>Zorgverlener:</strong> {encounter.serviceProvider?.display || 'N/A'}
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
