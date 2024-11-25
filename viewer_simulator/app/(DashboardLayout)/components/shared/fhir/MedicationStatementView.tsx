import React from 'react';
import { MedicationStatement } from 'fhir/r4';
import { Card, CardContent, CardHeader, Typography, List, ListItem, ListItemText } from '@mui/material';

interface MedicationStatementViewProps {
  medicationStatement: MedicationStatement;
}

export const MedicationStatementView: React.FC<MedicationStatementViewProps> = ({ medicationStatement }) => {
  return (
    <Card>
      <CardHeader title="Medication Statement" />
      <CardContent>
        <List>
          <ListItem>
            <ListItemText primary="Medication" secondary={medicationStatement.medicationCodeableConcept?.text || 'N/A'} />
          </ListItem>
          <ListItem>
            <ListItemText primary="Status" secondary={medicationStatement.status || 'N/A'} />
          </ListItem>
          <ListItem>
            <ListItemText primary="Effective" secondary={medicationStatement.effectiveDateTime || 'N/A'} />
          </ListItem>
          <ListItem>
            <ListItemText primary="Dosage" secondary={medicationStatement.dosage?.[0]?.text || 'N/A'} />
          </ListItem>
        </List>
      </CardContent>
    </Card>
  );
};

