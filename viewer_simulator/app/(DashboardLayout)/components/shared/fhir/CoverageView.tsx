import React from 'react';
import { Card, CardContent, Typography, List, ListItem, ListItemText } from '@mui/material';
import useBgzStore from '@/store/bgz-store';

export const CoverageView = () => {
  const { coverages } = useBgzStore();

  return (
    <Card>
      <CardContent>
        <Typography variant="h5" component="h2" gutterBottom>
          Insurance Information
        </Typography>
        <List>
          {coverages.map((coverage, index) => (
            <ListItem key={index}>
              <ListItemText
                primary={`Coverage ${index + 1}`}
                secondary={
                  <>
                    <Typography variant="body1" component="p" color="text.primary">
                      <strong>Status:</strong> {coverage.status || 'N/A'}
                    </Typography>
                    <br />
                    <Typography variant="body1" component="p" color="text.primary">
                      <strong>Type:</strong> {coverage.type?.text || 'N/A'}
                    </Typography>
                    <br />
                    <Typography variant="body1" component="p" color="text.primary">
                      <strong>Beneficiary:</strong> {coverage.beneficiary?.display || 'N/A'}
                    </Typography>
                    <br />
                    <Typography variant="body1" component="p" color="text.primary">
                      <strong>Payor:</strong> {coverage.payor?.[0]?.display || 'N/A'}
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
