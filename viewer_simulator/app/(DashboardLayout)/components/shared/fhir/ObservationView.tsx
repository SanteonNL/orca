import React from 'react';
import { Observation } from 'fhir/r4';
import { Card, CardContent, CardHeader, Typography, List, ListItem, ListItemText } from '@mui/material';

interface ObservationViewProps {
  observation: Observation;
}

export const ObservationView: React.FC<ObservationViewProps> = ({ observation }) => {
  return (
    <Card>
      <CardHeader title={observation.code?.text || 'Observation'} />
      <CardContent>
        <List>
          <ListItem>
            <ListItemText primary="Status" secondary={observation.status || 'N/A'} />
          </ListItem>
          <ListItem>
            <ListItemText primary="Category" secondary={observation.category?.[0]?.text || 'N/A'} />
          </ListItem>
          <ListItem>
            <ListItemText 
              primary="Value" 
              secondary={
                observation.valueQuantity 
                  ? `${observation.valueQuantity.value} ${observation.valueQuantity.unit}`
                  : observation.valueString || 'N/A'
              } 
            />
          </ListItem>
          <ListItem>
            <ListItemText primary="Effective Date" secondary={observation.effectiveDateTime || 'N/A'} />
          </ListItem>
        </List>
      </CardContent>
    </Card>
  );
};

