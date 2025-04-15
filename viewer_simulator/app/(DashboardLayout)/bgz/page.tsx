'use client';

import React, { useCallback, useEffect, useState } from 'react';
import {
    Box,
    Typography,
    MenuItem,
    Select,
    SelectChangeEvent,
} from '@mui/material';

import PageContainer from '../components/container/PageContainer';
import DashboardCard from '../components/shared/DashboardCard';
import BgzOverview from '../components/bgz/bgz-overview';

import useFetch from '@/app/useFetch';

export default function CarePlans() {
    const [roles, setRoles] = useState('default');
    const [name, setName] = useState<string | null>(null);
    const { data: endpoints } = useFetch<{ name: string; endpoint: string }[]>(
        `${process.env.NEXT_PUBLIC_BASE_PATH || ''}/api/bgz/endpoints`,
    );
    const updateEndpoint = useCallback((e: SelectChangeEvent<string>) => {
        setName(e.target.value);
    }, []);
    const updateRoles = useCallback((e: SelectChangeEvent<string>) => {
        setRoles(e.target.value);
    }, []);

    useEffect(() => {
        if (!name && endpoints?.length) {
            setName(endpoints[0].name);
        }
    }, [name, endpoints]);

    return (
        <Box sx={{ position: 'relative' }}>
            <PageContainer
                title="Care Plans"
                description="Use the CarePlanContributer to gather all health care data known about a patient. All organizations that are a member ofthe CareTeam will be queried."
            >
                <DashboardCard title="Care Plans">
                    <>
                        <Typography sx={{ mb: 2 }}>
                            Use the CarePlanContributer to gather all health
                            care data known about a patient. All organizations
                            that are a member ofthe CareTeam will be queried.
                        </Typography>
                        <div>
                            <Select
                                label="Role"
                                sx={{ mb: 2, mr: 1 }}
                                value={roles}
                                onChange={updateRoles}
                            >
                                <MenuItem value="default">
                                    Organization (default)
                                </MenuItem>
                                <MenuItem value="org1-generalpractitioner-harryarts">
                                    Arts Harry
                                </MenuItem>
                                <MenuItem value="123">Arts Claudia</MenuItem>
                                <MenuItem value="456">
                                    Verpleegkundige Roos
                                </MenuItem>
                            </Select>
                            {roles === 'default' &&
                                (endpoints?.length || 0) > 1 && (
                                    <Select
                                        label="Endpoint"
                                        sx={{ mb: 2 }}
                                        value={name || ''}
                                        onChange={updateEndpoint}
                                    >
                                        {endpoints?.map((endpoint) => (
                                            <MenuItem
                                                key={endpoint.name}
                                                value={endpoint.name}
                                            >
                                                {endpoint.name}
                                            </MenuItem>
                                        ))}
                                    </Select>
                                )}
                        </div>
                        {name && <BgzOverview name={name} roles={roles} />}
                    </>
                </DashboardCard>
            </PageContainer>
        </Box>
    );
}
