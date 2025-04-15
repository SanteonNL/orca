import React from 'react';
import { Card, CardContent, Typography, List, ListItem, ListItemText } from '@mui/material';
import useBgzStore from '@/store/bgz-store';

export const MedicationRequestView = () => {
    const { medicationRequests } = useBgzStore();
    console.log(medicationRequests)
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
                                disableTypography={true}
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
                                            <strong>Requester:</strong> {request.requester?.display}
                                        </Typography>
                                        <Typography component="p" variant="body1" color="text.primary">
                                            <strong>Reden:</strong> {request.reasonReference?.map(reason => reason.display).join(', ')}
                                        </Typography>
                                        <Typography component="p" variant="body1" color="text.primary">
                                            <strong>Dosering:</strong> {request.dosageInstruction?.map((dosage, i) => (
                                                <span key={i} style={{ marginLeft: 25, display: "block" }}>
                                                    <strong>Route:</strong> {dosage.route?.text} <br />
                                                    <strong>Hoeveelheid:</strong> {dosage.doseAndRate?.[0]?.doseQuantity?.value} {dosage.doseAndRate?.[0]?.doseQuantity?.unit} <br />
                                                    <strong>Frequentie:</strong> {dosage.timing?.repeat?.frequency} keer per {dosage.timing?.repeat?.period} {dosage.timing?.repeat?.periodUnit} <br />
                                                    <strong>Extra instructies:</strong> {dosage.additionalInstruction?.map(instr => instr.text).join(', ')}
                                                </span>
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
