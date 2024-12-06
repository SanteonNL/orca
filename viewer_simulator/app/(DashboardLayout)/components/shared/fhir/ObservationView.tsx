import React from 'react';
import { Observation } from 'fhir/r4';
import { Card, CardContent, CardHeader, Typography, List, ListItem, ListItemText } from '@mui/material';
import useBgzStore from '@/store/bgz-store';

interface ObservationViewProps {
  observation: Observation;
}

export const ObservationView = () => {
  const { observations } = useBgzStore();
  return (
    <Card>
      <CardContent>
        <Typography variant="h5" component="h2" gutterBottom>
          Observations
        </Typography>
        {observations.map((observation, index) => (
          <>
            <Typography component="p" variant="body1" color="text.primary">
              <strong>Patient:</strong> {observation.subject?.display || 'N/A'}
            </Typography>
            <Typography component="p" variant="body1" color="text.primary">
              <strong>Identifier:</strong> {observation.identifier?.[0]?.value || 'N/A'}
            </Typography>
            <Typography component="p" variant="body1" color="text.primary">
              <strong>Effective Period:</strong> {observation.effectivePeriod
                ? `${observation.effectivePeriod.start} - ${observation.effectivePeriod.end}`
                : 'N/A'}
            </Typography>
            <Typography component="p" variant="body1" color="text.primary">
              <strong>Value:</strong> {observation.valueCodeableConcept?.text || 'N/A'}
            </Typography>
            {observation.component?.map((component, compIndex) => (
              <div key={compIndex}>
                <Typography component="p" variant="body1" color="text.primary">
                  {component.code?.text || `Component ${compIndex + 1}`}: {component.valueQuantity
                    ? `${component.valueQuantity.value} ${component.valueQuantity.unit}`
                    : component.valueCodeableConcept?.text || 'N/A'}
                </Typography>
              </div>
            ))}
          </>
        ))}
      </CardContent>
    </Card >
  );
};

