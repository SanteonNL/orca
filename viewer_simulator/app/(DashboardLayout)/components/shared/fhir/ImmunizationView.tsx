import React from 'react';
import { Immunization } from 'fhir/r4';
import { Card, CardContent, CardHeader, Typography, List, ListItem, ListItemText } from '@mui/material';

interface ImmunizationViewProps {
  immunization: Immunization;
}

export const ImmunizationView: React.FC<ImmunizationViewProps> = ({ immunization }) => {
  return (
    <Card>
      <CardHeader title="Immunization" />
      <CardContent>
        <List>
          <ListItem>
            <ListItemText primary="Vaccine" secondary={immunization.vaccineCode?.text || 'N/A'} />
          </ListItem>
          <ListItem>
            <ListItemText primary="Status" secondary={immunization.status || 'N/A'} />
          </ListItem>
          <ListItem>
            <ListItemText primary="Date" secondary={immunization.occurrenceDateTime || 'N/A'} />
          </ListItem>
          <ListItem>
            <ListItemText primary="Route" secondary={immunization.route?.text || 'N/A'} />
          </ListItem>
        </List>
      </CardContent>
    </Card>
  );
};

