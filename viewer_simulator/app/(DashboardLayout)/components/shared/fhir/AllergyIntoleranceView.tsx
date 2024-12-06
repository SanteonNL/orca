import React from 'react';
import { AllergyIntolerance } from 'fhir/r4';
import { Card, CardContent, CardHeader, Typography, List, ListItem, ListItemText } from '@mui/material';

interface AllergyIntoleranceViewProps {
  allergyIntolerance: AllergyIntolerance;
}

export const AllergyIntoleranceView: React.FC<AllergyIntoleranceViewProps> = ({ allergyIntolerance }) => {
  return (
    <Card>
      <CardHeader title="Allergy Intolerance" />
      <CardContent>
        <List>
          <ListItem>
            <ListItemText primary="Code" secondary={allergyIntolerance.code?.text || 'N/A'} />
          </ListItem>
          <ListItem>
            <ListItemText primary="Type" secondary={allergyIntolerance.type || 'N/A'} />
          </ListItem>
          <ListItem>
            <ListItemText primary="Category" secondary={allergyIntolerance.category?.join(', ') || 'N/A'} />
          </ListItem>
          <ListItem>
            <ListItemText primary="Criticality" secondary={allergyIntolerance.criticality || 'N/A'} />
          </ListItem>
        </List>
      </CardContent>
    </Card>
  );
};

