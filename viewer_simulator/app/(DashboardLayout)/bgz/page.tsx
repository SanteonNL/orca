import React from 'react'
import PageContainer from '../components/container/PageContainer'
import { Box, Typography } from '@mui/material'
import DashboardCard from '../components/shared/DashboardCard'
import BgzOverview from '../components/bgz/bgz-overview'

export default function CarePlans() {

    return (
        <Box sx={{ position: 'relative' }}>
            <PageContainer
                title="Care Plans"
                description="Use the CarePlanContributer to gather all health care data known about a patient. All organizations that are a member ofthe CareTeam will be queried."
            >
                <DashboardCard title="Care Plans">
                    <>
                        <Typography sx={{ mb: 2 }}>Use the CarePlanContributer to gather all health care data known about a patient. All organizations that are a member ofthe CareTeam will be queried.</Typography>
                        <BgzOverview />
                    </>
                </DashboardCard>
            </PageContainer>
        </Box>
    )
}
