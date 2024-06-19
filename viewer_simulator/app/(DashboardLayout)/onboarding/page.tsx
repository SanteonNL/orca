import { Box, Typography } from '@mui/material';
import PageContainer from '@/app/(DashboardLayout)/components/container/PageContainer';
import DashboardCard from '@/app/(DashboardLayout)/components/shared/DashboardCard';
import AcceptedTaskOverview from '@/app/(DashboardLayout)/components/onboarding/accepted-task-overview';

const ServiceRequestsPage = () => {
  return (
    <Box sx={{ position: 'relative' }}>
      <PageContainer
        title="Onboarding"
        description="Shows all onboarding tasks that have been accepted by MSC"
      >
        <DashboardCard title="Onboarding">
          <>
            <Typography sx={{ mb: 2 }}>Shows all tasks that have been accepted by the MSC</Typography>
            <AcceptedTaskOverview />
          </>
        </DashboardCard>
      </PageContainer>
    </Box>
  );
};

export default ServiceRequestsPage;

