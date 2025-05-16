"use client"
import React from 'react';
import { DataGrid, GridColDef, GridToolbar } from '@mui/x-data-grid';
import { Tooltip } from '@mui/material';
import { IconEye, IconProgressBolt, IconProgressCheck, IconProgressHelp, IconProgressX } from '@tabler/icons-react';
import BgzDataViewer from './bgz-data-viewer';
import { CarePlan, CareTeam } from 'fhir/r4';

export type Row = {
  id: string,
  bsn: string,
  category: string,
  carePlan: CarePlan,
  status: string,
  lastUpdated: Date,
  careTeamMembers: string
}

interface Props {
    name: string,
    rows: Row[],
    loading?: boolean,
}

export default function BgzTable({ name, rows, loading }: Props) {
    console.log('Entries: ' + rows.length)
    console.log(rows)
    const columns: GridColDef[] = [

        { field: 'bsn', headerName: 'BSN', flex: 1 },
        { field: 'category', headerName: 'Category', flex: 2 },
        { field: 'careTeamMembers', headerName: 'Potential Data Holders', flex: 2 },
        {
            field: 'status',
            headerName: 'Status',
            flex: 1,
            renderCell: (params) => {
                switch (params.row.status) {
                    case "requested": return <Tooltip title={params.row.status}><IconProgressHelp /></Tooltip>
                    case "ready": return <Tooltip title={params.row.status}><IconEye /></Tooltip>
                    case "active": return <Tooltip title={params.row.status}><IconProgressBolt /></Tooltip>
                    case "cancelled": return <Tooltip title={params.row.status}><IconProgressX /></Tooltip>
                    case "completed": return <Tooltip title={params.row.status}><IconProgressCheck /></Tooltip>
                    default: return params.row.status
                }
            }
        },
        {
            field: 'medicalRecordViewer',
            headerName: 'Medical Records',
            flex: 1,
            renderCell: (params: { row: Row }) => {
                return <BgzDataViewer name={name} carePlan={params.row.carePlan} />
            }
        },
        { field: 'lastUpdated', headerName: 'Last Updated', type: 'dateTime', flex: 2 },
    ];

    return (
        <div>
            <DataGrid
                loading={loading}
                rows={rows}
                columns={columns}
                components={{ Toolbar: GridToolbar }}
                autoHeight
                pageSize={10}
                rowsPerPageOptions={[10]}
                componentsProps={{
                    toolbar: {
                        showQuickFilter: true,
                    },
                }}
                initialState={{
                    sorting: {
                        sortModel: [{ field: 'lastUpdated', sort: 'desc' }]
                    },
                }}
            />
        </div>
    );
}