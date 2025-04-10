import React from 'react';
import { Card, CardContent, Typography, List, ListItem, ListItemText } from '@mui/material';
import useBgzStore from '@/store/bgz-store';

export const ProcedureRequestView = () => {
    const { procedureRequests } = useBgzStore();
    return (
        <Card>
            <CardContent>
                <Typography variant="h2" component="h2" gutterBottom>
                    Procedureverzoeken
                </Typography>
                <List>
                    {procedureRequests.map((request, index) => (
                        <ListItem key={request.id || index}>
                            <ListItemText
                                primary={`Procedureverzoek ${index + 1}`}
                                secondary={
                                    <>
                                        <Typography component="p" variant="body1" color="text.primary">
                                            <strong>Status:</strong> {request.status}
                                        </Typography>
                                        <Typography component="p" variant="body1" color="text.primary">
                                            <strong>Intentie:</strong> {request.intent}
                                        </Typography>
                                        <Typography component="p" variant="body1" color="text.primary">
                                            <strong>Code:</strong> {request.code?.text}
                                        </Typography>
                                        <Typography component="p" variant="body1" color="text.primary">
                                            <strong>Categorie:</strong> {request.category?.[0]?.text}
                                        </Typography>
                                        <Typography component="p" variant="body1" color="text.primary">
                                            <strong>Patiënt:</strong> {request.subject?.display}
                                        </Typography>
                                        <Typography component="p" variant="body1" color="text.primary">
                                            <strong>Uitvoerder:</strong> {request.performer?.display}
                                        </Typography>
                                        <Typography component="p" variant="body1" color="text.primary">
                                            <strong>Lichaamsdeel:</strong> {request.bodySite?.[0]?.text}
                                        </Typography>
                                        <Typography component="p" variant="body1" color="text.primary">
                                            <strong>Aanvrager:</strong> {request.requester?.agent?.display}
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
