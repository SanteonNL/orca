import React from 'react';
import { Card, CardContent, Typography } from '@mui/material';
import useBgzStore from '@/store/bgz-store';

export const NutritionOrderView = () => {
    const { nutritionOrders } = useBgzStore();
    return nutritionOrders.map((order, index) => (
        <Card key={index} style={{ marginBottom: '1rem' }}>
            <CardContent>
                <Typography variant="h5" component="h2" color="text.primary" gutterBottom>
                    Nutrition Order {index + 1}
                </Typography>
                <Typography variant="body1" component="p" color="text.primary">
                    <strong>Date:</strong> {order.dateTime}
                </Typography>
                <Typography variant="body1" component="p" color="text.primary">
                    <strong>Diet:</strong> {order.oralDiet?.type?.[0]?.text}
                </Typography>
                <Typography variant="body1" component="p" color="text.primary">
                    <strong>Patient:</strong> {order.patient?.display}
                </Typography>
                <Typography variant="body1" component="p" color="text.primary">
                    <strong>Identifier:</strong> {order.identifier?.[0]?.value}
                </Typography>
            </CardContent>
        </Card>
    ));
};
