import { Chip, Card, CardContent, CardHeader, Typography, Table, TableBody, TableCell, TableHead, TableRow, Paper } from '@mui/material';

export const DataDisplay = ({ label, value }: { label: string; value: any }) => {
  if (value === undefined || value === null) return null;

  if (typeof value === "object" && !Array.isArray(value)) {
    return (
      <Card sx={{ mb: 2 }}>
        <CardHeader title={label} />
        <CardContent>
          {Object.entries(value).map(([key, val]) => (
            <DataDisplay key={key} label={key} value={val} />
          ))}
        </CardContent>
      </Card>
    );
  }

  if (Array.isArray(value)) {
    return (
      <Card sx={{ mb: 2 }}>
        <CardHeader title={label} />
        <CardContent>
          <Table size="small">
            <TableHead>
              <TableRow>
                {Object.keys(value[0] || {}).map((key) => (
                  <TableCell key={key}>{key}</TableCell>
                ))}
              </TableRow>
            </TableHead>
            <TableBody>
              {value.map((item, index) => (
                <TableRow key={index}>
                  {Object.values(item).map((val: any, i) => (
                    <TableCell key={i}>
                      {typeof val === "object" ? JSON.stringify(val) : String(val)}
                    </TableCell>
                  ))}
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </CardContent>
      </Card>
    );
  }

  return (
    <div className="mb-2">
      <span className="font-semibold">{label}: </span>
      {typeof value === "boolean" ? (
        <Chip label={value ? "Yes" : "No"} color={value ? "primary" : "default"} size="small" />
      ) : (
        <span>{value}</span>
      )}
    </div>
  );
};

