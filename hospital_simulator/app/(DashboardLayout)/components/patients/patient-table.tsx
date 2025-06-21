"use client"
import React from 'react';
import { DataGrid, GridColDef, GridToolbar } from '@mui/x-data-grid';
import {HumanName, Identifier} from "fhir/r4";
import {IconCheckupList} from "@tabler/icons-react";
import {FormatHumanName} from "@/utils/fhir";

interface Props {
    rows: PatientDetails[]
}

interface PatientDetails {
    id: string
    lastUpdated: Date
    primaryIdentifier: Identifier
    name: HumanName
    gender: string
}

const PatientTable: React.FC<Props> = ({ rows }) => {
    const columns: GridColDef[] = [
        {
            field: 'id',
            headerName: 'BSN',
        },
        { field: 'lastUpdated', headerName: 'Last Updated', type: 'dateTime', flex: 1 },
        {
            field: 'name',
            headerName: 'Name',
            flex: 2,
            renderCell: (params) => {
                return FormatHumanName(params.value as HumanName)
            }
        },
        { field: 'gender', headerName: 'Gender', flex: 1 },
        {
            headerName: 'Service Requests',
            flex: 1,
            field: 'primaryIdentifier',
            renderCell: params => {
                const s = encodeURIComponent(`${params.value.system ?? ""}|${params.value.value ?? ""}`);
                return (
                    <a href={process.env.NEXT_PUBLIC_BASE_PATH + `/service-requests?patient=${s}`} style={{ textDecoration: 'none', color: 'inherit' }}>
                        <IconCheckupList />
                    </a>
                );
            }
        },
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

export default PatientTable