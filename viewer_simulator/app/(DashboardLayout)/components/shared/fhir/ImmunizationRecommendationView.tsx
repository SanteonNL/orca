import React from 'react';
import { Card, CardContent, CardHeader, Typography } from '@mui/material';
import useBgzStore from '@/store/bgz-store';

export const ImmunizationRecommendationView = () => {

    const { immunizationRecommendations } = useBgzStore();

    return immunizationRecommendations.map((immunizationRecommendation, index) => (
        <Card key={index} style={{ marginBottom: '1rem' }}>
            <CardContent>
                <Typography variant="h5" component="h2" gutterBottom>
                    Immunization Recommendation {index + 1}
                </Typography>
                <Typography component="p" variant="body1" color="text.primary">
                    <strong>Patient:</strong> {immunizationRecommendation.patient.reference}
                </Typography>
                {immunizationRecommendation.recommendation.map((rec, recIndex) => (
                    <Card key={recIndex} style={{ marginBottom: '0.5rem' }}>
                        <CardHeader title={`Recommendation ${recIndex + 1}`} />
                        <CardContent>
                            <Typography component="p" variant="body1" color="text.primary">
                                <strong>Vaccine:</strong> {rec.vaccineCode?.[0]?.text}
                            </Typography>
                            <Typography component="p" variant="body1" color="text.primary">
                                <strong>Forecast Status:</strong> {rec.forecastStatus.text}
                            </Typography>
                        </CardContent>
                    </Card>
                ))}
            </CardContent>
        </Card>
    ));
};

