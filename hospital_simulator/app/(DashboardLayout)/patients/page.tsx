import { Box, Typography } from '@mui/material';
import PageContainer from '@/app/(DashboardLayout)/components/container/PageContainer';
import DashboardCard from '@/app/(DashboardLayout)/components/shared/DashboardCard';
import PatientOverview from '@/app/(DashboardLayout)/components/patients/patient-overview';

const PatientsPage = () => {
  return (
    <Box sx={{ position: 'relative' }}>
      <PageContainer
        title="Patients"
        description="Shows all patients"
      >
        <DashboardCard title="Patients">
          <>
            <Typography>Shows all patients</Typography>
            <PatientOverview />
          </>
        </DashboardCard>
      </PageContainer>
    </Box>
  );
};

export default PatientsPage;

