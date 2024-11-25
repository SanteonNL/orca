import React from 'react';
import { Procedure } from 'fhir/r4';
import { Card, CardContent, CardHeader, Typography, List, ListItem, ListItemText } from '@mui/material';

interface ProcedureViewProps {
  procedure: Procedure;
}

export const ProcedureView: React.FC<ProcedureViewProps> = ({ procedure }) => {
  return (
    <Card>
      <CardHeader title="Procedure" />
      <CardContent>
        <List>
          <ListItem>
            <ListItemText primary="Code" secondary={procedure.code?.text || 'N/A'} />
          </ListItem>
          <ListItem>
            <ListItemText primary="Status" secondary={procedure.status || 'N/A'} />
          </ListItem>
          <ListItem>
            <ListItemText primary="Performed" secondary={procedure.performedDateTime || 'N/A'} />
          </ListItem>
          <ListItem>
            <ListItemText primary="Category" secondary={procedure.category?.text || 'N/A'} />
          </ListItem>
        </List>
      </CardContent>
    </Card>
  );
};

