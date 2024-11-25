import React from 'react';
import { Coverage } from 'fhir/r4';
import { Card, CardContent, CardHeader, Typography, List, ListItem, ListItemText } from '@mui/material';

interface CoverageViewProps {
  coverage: Coverage;
}

export const CoverageView: React.FC<CoverageViewProps> = ({ coverage }) => {
  return (
    <Card>
      <CardHeader title="Insurance Information" />
      <CardContent>
        <List>
          <ListItem>
            <ListItemText primary="Status" secondary={coverage.status || 'N/A'} />
          </ListItem>
          <ListItem>
            <ListItemText primary="Type" secondary={coverage.type?.text || 'N/A'} />
          </ListItem>
          <ListItem>
            <ListItemText primary="Beneficiary" secondary={coverage.beneficiary?.display || 'N/A'} />
          </ListItem>
          <ListItem>
            <ListItemText primary="Payor" secondary={coverage.payor?.[0]?.display || 'N/A'} />
          </ListItem>
        </List>
      </CardContent>
    </Card>
  );
};

