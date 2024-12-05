import React from 'react';
import { Condition } from 'fhir/r4';
import { Card, CardContent, CardHeader, Typography, List, ListItem, ListItemText } from '@mui/material';
import useBgzStore from '@/store/bgz-store';

interface ConditionViewProps {
  condition: Condition;
}

export const ConditionView = () => {
  const { conditions } = useBgzStore();
  return (
    <Card>
      <CardHeader title="Condition" />
      <CardContent>
        <Typography variant="h5" component="h2" gutterBottom>
          Conditions
        </Typography>
        <List>
          {conditions.map((condition, index) => (
            <ListItem key={index}>
              <ListItemText
                primary={condition.code?.text || `Condition ${index + 1}`}
                secondary={
                  <>
                    <Typography variant="body1" component="p" color="text.primary">
                      <strong>Onderwerp:</strong> {condition.subject?.display || 'Onbekend'}
                    </Typography>
                    <Typography variant="body1" component="p" color="text.primary">
                      <strong>Klinische status:</strong> {condition.clinicalStatus || 'Onbekend'}
                    </Typography>
                    <Typography variant="body1" component="p" color="text.primary">
                      <strong>Verificatiestatus:</strong> {condition.verificationStatus || 'Onbekend'}
                    </Typography>
                    <Typography variant="body1" component="p" color="text.primary">
                      <strong>Categorie:</strong> {condition.category?.[0]?.text || 'Onbekend'}
                    </Typography>
                    <Typography variant="body1" component="p" color="text.primary">
                      <strong>Code:</strong> {condition.code?.text || 'Onbekend'}
                    </Typography>
                    <Typography variant="body1" component="p" color="text.primary">
                      <strong>Begindatum/tijd:</strong> {condition.onsetDateTime || 'Onbekend'}
                    </Typography>
                    <Typography variant="body1" component="p" color="text.primary">
                      <strong>Lichaamslocatie:</strong> {condition.bodySite?.[0]?.extension?.[0]?.valueCodeableConcept?.text || 'Onbekend'}
                    </Typography>
                    <Typography variant="body1" component="p" color="text.primary">
                      <strong>Lateralisatie:</strong> {condition.bodySite?.[0]?.extension?.[0]?.valueCodeableConcept?.coding?.[0]?.display || 'Onbekend'}
                    </Typography>
                    <Typography variant="body1" component="p" color="text.primary">
                      <strong>Verantwoordelijke:</strong> {condition.asserter?.display || 'Onbekend'}
                    </Typography>
                    <Typography variant="body1" component="p" color="text.primary">
                      <strong>Opmerking:</strong> {condition.note?.[0]?.text || 'Geen opmerkingen'}
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

