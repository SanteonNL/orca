import React from 'react';
import { Card, CardContent, Typography, List, ListItem, ListItemText } from '@mui/material';
import useBgzStore from '@/store/bgz-store';

export const FlagView = () => {
    const { flags } = useBgzStore();
    return (
        <Card>
            <CardContent>
                <Typography variant="h5" component="h2" gutterBottom>
                    Flags
                </Typography>
                <List>
                    {flags.map((flag, index) => (
                        <ListItem key={flag.id || index}>
                            <ListItemText
                                primary={flag.code?.text}
                                secondary={
                                    <>
                                        <Typography component="p" variant="body1" color="text.primary">
                                            <strong>Status:</strong> {flag.status}
                                        </Typography>
                                        <Typography component="p" variant="body1" color="text.primary">
                                            <strong>Categorie:</strong> {flag.category?.[0]?.text}
                                        </Typography>
                                        <Typography component="p" variant="body1" color="text.primary">
                                            <strong>Code:</strong> {flag.code?.text}
                                        </Typography>
                                        <Typography component="p" variant="body1" color="text.primary">
                                            <strong>Onderwerp:</strong> {flag.subject?.display}
                                        </Typography>
                                        <Typography component="p" variant="body1" color="text.primary">
                                            <strong>Periode start:</strong> {flag.period?.start}
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
