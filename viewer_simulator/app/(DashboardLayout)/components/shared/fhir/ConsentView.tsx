import React from 'react';
import { Consent } from 'fhir/r4';
import { Card, CardContent, CardHeader, Typography, List, ListItem, ListItemText } from '@mui/material';

interface ConsentViewProps {
  consent: Consent;
}

export const ConsentView: React.FC<ConsentViewProps> = ({ consent }) => {
  return (
    <Card>
      <CardHeader title={consent.category?.[0]?.text || 'Consent'} />
      <CardContent>
        <List>
          <ListItem>
            <ListItemText primary="Status" secondary={consent.status || 'N/A'} />
          </ListItem>
          <ListItem>
            <ListItemText primary="Scope" secondary={consent.scope?.text || 'N/A'} />
          </ListItem>
          <ListItem>
            <ListItemText primary="Category" secondary={consent.category?.[0]?.coding?.[0]?.display || 'N/A'} />
          </ListItem>
          <ListItem>
            <ListItemText primary="Patient" secondary={consent.patient?.display || 'N/A'} />
          </ListItem>
        </List>
      </CardContent>
    </Card>
  );
};

