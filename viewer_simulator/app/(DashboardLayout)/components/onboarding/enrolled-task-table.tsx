"use client"
import React, { useState } from 'react';
import { DataGrid, GridColDef, GridToolbar } from '@mui/x-data-grid';
import { Button, Tooltip } from '@mui/material';
import { IconEye, IconProgressBolt, IconProgressCheck, IconProgressHelp, IconProgressX } from '@tabler/icons-react';
import ViewTaskOutput from './view-task-output';

interface Props {
    rows: any[]
}

const EnrolledTaskTable: React.FC<Props> = ({ rows }) => {

    const columns: GridColDef[] = [

        { field: 'requesterUra', headerName: 'Requester URA', flex: 2 },
        { field: 'requesterName', headerName: 'Requester Name', flex: 2 },
        { field: 'practitionerRoleIdentifiers', headerName: 'Requesting PractitionerRole Identifiers', flex: 2 },
        { field: 'performerUra', headerName: 'Performer URA', flex: 2 },
        { field: 'performerName', headerName: 'Performer Name', flex: 2 },
        { field: 'patientBsn', headerName: 'Patient BSN', flex: 2 },
        { field: 'serviceRequest', headerName: 'Service', flex: 3 },
        { field: 'condition', headerName: 'Care Path', flex: 3 },
        {
            field: 'status',
            headerName: 'Status',
            flex: 1,
            renderCell: (params) => {
                switch (params.row.status) {
                    case "requested": return <Tooltip title={params.row.status}><IconProgressHelp /></Tooltip>
                    case "ready": return <Tooltip title={params.row.status}><IconEye /></Tooltip>
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

                if (!params.row.isSubtask) return <></>

                return <ViewTaskOutput task={params.row.task} />
            }
        }
    ];

    return (
        <div>
            <DataGrid
                rows={rows}
                columns={columns}
                components={{ Toolbar: GridToolbar }}
                autoHeight
                componentsProps={{
                    toolbar: {
                        showQuickFilter: true,
                    },
                }}
                initialState={{
                    sorting: {
                        sortModel: [{ field: 'lastUpdated', sort: 'desc' }]
                    },
                    pagination: {
                        pageSize: 10,
                    },
                }}
            />
        </div>
    );
}

export default EnrolledTaskTable