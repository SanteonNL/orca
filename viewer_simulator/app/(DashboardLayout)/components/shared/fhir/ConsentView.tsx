import React from 'react';
import { Card, CardContent, Typography, List, ListItem, ListItemText } from '@mui/material';
import useBgzStore from '@/store/bgz-store';


export const ConsentView = () => {
  const { consents } = useBgzStore();
  return (
    <Card>
      <CardContent>
        <Typography variant="h5" component="h2" gutterBottom>
          Toestemmingen
        </Typography>
        <List>
          {consents.map((consent, index) => (
            <ListItem key={index}>
              <ListItemText
                disableTypography={true}
                primary={consent.category?.[0]?.text || `Toestemming ${index + 1}`}
                secondary={
                  <>
                    <Typography variant="body1" component="p" color="text.primary">
                      <strong>Status:</strong> {consent.status || 'N/A'}
                    </Typography>
                    <Typography variant="body1" component="p" color="text.primary">
                      <strong>Categorie:</strong> {consent.category?.[0]?.coding?.[0]?.display || 'N/A'}
                    </Typography>
                    <Typography variant="body1" component="p" color="text.primary">
                      <strong>Patiënt:</strong> {consent.patient?.display || 'N/A'}
                    </Typography>
                    <Typography variant="body1" component="p" color="text.primary">
                      <strong>Behandeling:</strong> {consent.extension?.find(ext => ext.url === 'http://nictiz.nl/fhir/StructureDefinition/zib-TreatmentDirective-Treatment')?.valueCodeableConcept?.text || 'N/A'}
                    </Typography>
                    <Typography variant="body1" component="p" color="text.primary">
                      <strong>Behandeling toegestaan:</strong> {consent.modifierExtension?.find(ext => ext.url === 'http://nictiz.nl/fhir/StructureDefinition/zib-TreatmentDirective-TreatmentPermitted')?.valueCodeableConcept?.text || 'N/A'}
                    </Typography>
                    <Typography variant="body1" component="p" color="text.primary">
                      <strong>Startdatum:</strong> {consent.dateTime || 'N/A'}
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
