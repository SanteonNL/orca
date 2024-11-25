import React from 'react';
import { Condition } from 'fhir/r4';
import { Card, CardContent, CardHeader, Typography, List, ListItem, ListItemText } from '@mui/material';

interface ConditionViewProps {
  condition: Condition;
}

export const ConditionView: React.FC<ConditionViewProps> = ({ condition }) => {
  return (
    <Card>
      <CardHeader title="Condition" />
      <CardContent>
        <List>
          <ListItem>
            <ListItemText primary="Code" secondary={condition.code?.text || 'N/A'} />
          </ListItem>
          <ListItem>
            <ListItemText primary="Clinical Status" secondary={condition.clinicalStatus?.text || 'N/A'} />
          </ListItem>
          <ListItem>
            <ListItemText primary="Verification Status" secondary={condition.verificationStatus?.text || 'N/A'} />
          </ListItem>
          <ListItem>
            <ListItemText primary="Onset" secondary={condition.onsetDateTime || 'N/A'} />
          </ListItem>
        </List>
      </CardContent>
    </Card>
  );
};

