import { Box, Typography } from '@mui/material';
import PageContainer from '@/app/(DashboardLayout)/components/container/PageContainer';
import DashboardCard from '@/app/(DashboardLayout)/components/shared/DashboardCard';
import ServiceRequestOverview from '@/app/(DashboardLayout)/components/service-requests/service-request-overview';

const ServiceRequestsPage = () => {
  return (
    <Box sx={{ position: 'relative' }}>
      <PageContainer
        title="Service Requests"
        description="Shows all service requests, create new requests and view its status"
      >
        <DashboardCard title="Service Requests">
          <>
            <Typography>Shows all service requests, create new requests and view its status</Typography>
            <ServiceRequestOverview />
          </>
        </DashboardCard>
      </PageContainer>
    </Box>
  );
};

export default ServiceRequestsPage;

