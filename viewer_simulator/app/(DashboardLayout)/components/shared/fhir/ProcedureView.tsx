import React from 'react';
import { Card, CardContent, CardHeader, Typography, List, ListItem, ListItemText } from '@mui/material';
import useBgzStore from '@/store/bgz-store';

export const ProcedureView = () => {
  const { procedures } = useBgzStore();
  return (
    <Card>
      <CardContent>
        <Typography variant="h5" component="h2" gutterBottom>
          Procedures
        </Typography>
        <List>
          {procedures.map((procedure, index) => (
            <ListItem key={index}>
              <ListItemText
                disableTypography={true}
                primary={procedure.code?.text || `Procedure ${index + 1}`}
                secondary={
                  <>
                    <Typography component="p" variant="body1" color="text.primary">
                      <strong>Patient:</strong> {procedure.subject?.display || 'N/A'}
                    </Typography>
                    <Typography component="p" variant="body1" color="text.primary">
                      <strong>Code:</strong> {procedure.code?.text || 'N/A'}
                    </Typography>
                    <Typography component="p" variant="body1" color="text.primary">
                      <strong>Category:</strong> {procedure.category?.coding?.[0]?.display || 'N/A'}
                    </Typography>
                    <Typography component="p" variant="body1" color="text.primary">
                      <strong>Status:</strong> {procedure.status || 'N/A'}
                    </Typography>
                    <Typography component="p" variant="body1" color="text.primary">
                      <strong>Performed:</strong> {procedure.performedPeriod ? `${procedure.performedPeriod.start} to ${procedure.performedPeriod.end}` : 'N/A'}
                    </Typography>
                    <Typography component="p" variant="body1" color="text.primary">
                      <strong>Body Site:</strong> {procedure.bodySite?.[0]?.coding?.[0]?.display || 'N/A'}
                    </Typography>
                    <Typography component="p" variant="body1" color="text.primary">
                      <strong>Laterality:</strong> {procedure.bodySite?.[0]?.extension?.[0]?.valueCodeableConcept?.text || 'N/A'}
                    </Typography>
                    <Typography component="p" variant="body1" color="text.primary">
                      <strong>Performer:</strong> {procedure.performer?.[0]?.actor?.display || 'N/A'}
                    </Typography>
                    <Typography component="p" variant="body1" color="text.primary">
                      <strong>Location:</strong> {procedure.location?.display || 'N/A'}
                    </Typography>
                    <Typography component="p" variant="body1" color="text.primary">
                      <strong>Note:</strong> {procedure.note?.[0]?.text || 'N/A'}
                    </Typography>
                    <Typography component="p" variant="body1" color="text.primary">
                      <strong>Focal Device:</strong> {procedure.focalDevice?.[0]?.manipulated?.display || 'N/A'}
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

