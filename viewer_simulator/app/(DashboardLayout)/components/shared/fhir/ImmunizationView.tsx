import React from 'react';
import { Card, CardContent, Typography, List, ListItem, ListItemText } from '@mui/material';
import useBgzStore from '@/store/bgz-store';

export const ImmunizationView = () => {

  const { immunizations } = useBgzStore();

  return (
    <List>
      {immunizations.map((immunization, index) => (
        <Card key={index} style={{ marginBottom: '1rem' }}>
          <CardContent>
            <Typography variant="h6" component="h3">
              {immunization.vaccineCode?.text || `Vaccinatie ${index + 1}`}
            </Typography>
            <Typography component="p" variant="body1" color="text.primary">
              <strong>Status:</strong> {immunization.status === 'completed' ? 'Voltooid' : immunization.status}
            </Typography>
            <Typography component="p" variant="body1" color="text.primary">
              <strong>Datum:</strong> {immunization?.occurrenceDateTime ? new Date(immunization?.occurrenceDateTime).toLocaleDateString('nl-NL') : 'N/A'}
            </Typography>
            <Typography variant="body1" component="p">
              <strong>Opmerking:</strong> {immunization.note?.[0]?.text || 'Geen opmerkingen'}
            </Typography>
            <Typography component="p" variant="body1" color="text.primary">
              <strong>Patiënt:</strong> {immunization.patient?.display}
            </Typography>
            <Typography component="p" variant="body1" color="text.primary">
              <strong>Auteur:</strong> {immunization.note?.[0]?.authorReference?.display}
            </Typography>
          </CardContent>
        </Card>
      ))}
    </List>
  );
};
