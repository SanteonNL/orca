import React from 'react';
import { Card, CardContent, Typography, List, ListItem, ListItemText } from '@mui/material';
import useBgzStore from '@/store/bgz-store';

export const MedicationRequestView = () => {
    const { medicationRequests } = useBgzStore();
    return (
        <Card>
            <CardContent>
                <Typography variant="h5" component="h2" gutterBottom>
                    Medicatieverzoeken
                </Typography>
                <List>
                    {medicationRequests.map((request, index) => (
                        <ListItem key={request.id || index}>
                            <ListItemText
                                primary={`Medicatieverzoek ${index + 1}`}
                                secondary={
                                    <>
                                        <Typography component="p" variant="body1" color="text.primary">
                                            <strong>Status:</strong> {request.status}
                                        </Typography>
                                        <Typography component="p" variant="body1" color="text.primary">
                                            <strong>Intentie:</strong> {request.intent}
                                        </Typography>
                                        <Typography component="p" variant="body1" color="text.primary">
                                            <strong>Medicatie:</strong> {request.medicationCodeableConcept?.text || 'N.v.t.'}
                                        </Typography>
                                        <Typography component="p" variant="body1" color="text.primary">
                                            <strong>Datum:</strong> {request.authoredOn}
                                        </Typography>
                                        <Typography component="p" variant="body1" color="text.primary">
                                            <strong>Periode van gebruik:</strong> {request.extension?.find(ext => ext.url === 'http://nictiz.nl/fhir/StructureDefinition/zib-Medication-PeriodOfUse')?.valuePeriod?.start} - {request.extension?.find(ext => ext.url === 'http://nictiz.nl/fhir/StructureDefinition/zib-Medication-PeriodOfUse')?.valuePeriod?.end}
                                        </Typography>
                                        <Typography component="p" variant="body1" color="text.primary">
                                            <strong>Stop type:</strong> {request.modifierExtension?.find(ext => ext.url === 'http://nictiz.nl/fhir/StructureDefinition/zib-Medication-StopType')?.valueCodeableConcept?.text}
                                        </Typography>
                                        <Typography component="p" variant="body1" color="text.primary">
                                            <strong>Requester:</strong> {request.requester?.agent?.display}
                                        </Typography>
                                        <Typography component="p" variant="body1" color="text.primary">
                                            <strong>Reden:</strong> {request.reasonReference?.map(reason => reason.display).join(', ')}
                                        </Typography>
                                        <Typography component="p" variant="body1" color="text.primary">
                                            <strong>Dosering:</strong> {request.dosageInstruction?.map(dosage => (
                                                <div key={dosage.sequence} style={{ marginLeft: 25 }}>
                                                    <strong>Route:</strong> {dosage.route?.text} <br />
                                                    <strong>Hoeveelheid:</strong> {dosage.doseQuantity?.value} {dosage.doseQuantity?.unit} <br />
                                                    <strong>Frequentie:</strong> {dosage.timing?.repeat?.frequency} keer per {dosage.timing?.repeat?.period} {dosage.timing?.repeat?.periodUnit} <br />
                                                    <strong>Extra instructies:</strong> {dosage.additionalInstruction?.map(instr => instr.text).join(', ')}
                                                </div>
                                            ))}
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
