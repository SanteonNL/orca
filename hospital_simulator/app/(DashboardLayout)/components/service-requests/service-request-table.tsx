"use client"
import React from 'react';
import { DataGrid, GridColDef, GridToolbar } from '@mui/x-data-grid';
import { Tooltip } from '@mui/material';
import { IconProgressBolt, IconProgressCheck, IconProgressHelp, IconProgressX } from '@tabler/icons-react';
import { useRouter } from 'next/navigation';
import EnrollmentPopup from './enrollment-popup';

interface Props {
    rows: any[]
}

const ServiceRequestTable: React.FC<Props> = ({ rows }) => {
    const router = useRouter()

    const enrollServiceRequest = async (row: any) => {

        const resp = await fetch(`${process.env.NEXT_PUBLIC_BASE_PATH || ''}/api/fhir/ServiceRequest/${row.id}`, {
            method: "PATCH",
            headers: { "Content-Type": "application/json-patch+json" },
            body: JSON.stringify(
                [
                    {
                        "op": "replace",
                        "path": "/status",
                        "value": "active"
                    }
                ]
            )
        })

        if (resp.ok) {
            router.refresh()
        }
    }

    const columns: GridColDef[] = [
        { field: 'lastUpdated', headerName: 'Last Updated', type: 'dateTime', flex: 2 },
        { field: 'title', headerName: 'Title', flex: 3 },
        { field: 'patient', headerName: 'Patient', flex: 2 },
        {
            field: 'status',
            headerName: 'Status',
            flex: 1,
            renderCell: (params) => {
                switch (params.row.status) {
                    case "draft": return <Tooltip title={params.row.status}><IconProgressHelp /></Tooltip>
                    case "active": return <Tooltip title={params.row.status}><IconProgressBolt /></Tooltip>
                    case "cancelled": return <Tooltip title={params.row.status}><IconProgressX /></Tooltip>
                    case "completed": return <Tooltip title={params.row.status}><IconProgressCheck /></Tooltip>
                    default: return params.row.status
                }
            }
        },
        {
            field: 'action',
            headerName: 'Enroll',
            sortable: false,
            renderCell: (params) => {
                if (params.row.status !== "draft") return <></>

                return <EnrollmentPopup patientId={params.row.patientId} serviceRequestId={params.row.id} callback={() => enrollServiceRequest(params.row)} />;
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

export default ServiceRequestTable