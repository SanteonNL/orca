"use client"
import React, { useState } from 'react';
import { DataGrid, GridColDef, GridToolbar } from '@mui/x-data-grid';
import { Button, Tooltip } from '@mui/material';
import { IconCloudDataConnection, IconEye, IconProgress, IconProgressBolt, IconProgressCheck, IconProgressHelp, IconProgressX } from '@tabler/icons-react';
import ViewTaskOutput from './view-task-output';

interface Props {
    rows: any[]
}

const EnrolledTaskTable: React.FC<Props> = ({ rows }) => {

    const columns: GridColDef[] = [

        { field: 'hospitalUra', headerName: 'Hospital URA', flex: 1 },
        { field: 'hospitalName', headerName: 'Hospital Name', flex: 2 },
        { field: 'patientBsn', headerName: 'Patient BSN', flex: 2 },
        { field: 'careplan', headerName: 'Careplan', flex: 2 },
        {
            field: 'status',
            headerName: 'Status',
            flex: 1,
            renderCell: (params) => {
                switch (params.row.status) {
                    case "requested": return <Tooltip title={params.row.status}><IconProgressHelp /></Tooltip>
                    case "accepted": return <Tooltip title={params.row.status}><IconProgressBolt /></Tooltip>
                    case "cancelled": return <Tooltip title={params.row.status}><IconProgressX /></Tooltip>
                    case "completed": return <Tooltip title={params.row.status}><IconProgressCheck /></Tooltip>
                    default: return params.row.status
                }
            }
        },
        { field: 'lastUpdated', headerName: 'Last Updated', type: 'dateTime', flex: 2 },
        {
            field: 'taskOutput',
            headerName: 'Task Output',
            flex: 1,
            renderCell: (params) => {
                return <ViewTaskOutput task={params.row.task} />
            }
        }
    ];

    return (
        <div>
            <DataGrid
                rows={rows}
                columns={columns}
                slots={{ toolbar: GridToolbar }}
                autoHeight
                slotProps={{
                    toolbar: {
                        showQuickFilter: true,
                    },
                }}
                initialState={{
                    sorting: {
                        sortModel: [{ field: 'lastUpdated', sort: 'desc' }]
                    },
                    pagination: {
                        paginationModel: {
                            pageSize: 10,
                        },
                    },
                }}
            />
        </div>
    );
}

export default EnrolledTaskTable