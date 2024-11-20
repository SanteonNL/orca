import React, { useState } from 'react'

import { IconEyeSearch } from '@tabler/icons-react';
import { CarePlan, CareTeam } from 'fhir/r4';
import { getBsn } from '@/utils/fhirUtils';
import Dialog from '@mui/material/Dialog';
import AppBar from '@mui/material/AppBar';
import Toolbar from '@mui/material/Toolbar';
import Typography from '@mui/material/Typography';
import CloseIcon from '@mui/icons-material/Close';
import Slide from '@mui/material/Slide';
import { TransitionProps } from '@mui/material/transitions';
import { Box, IconButton } from '@mui/material';
import { getBgzData } from './actions';

const Transition = React.forwardRef(function Transition(
    props: TransitionProps & {
        children: React.ReactElement;
    },
    ref: React.Ref<unknown>,
) {
    return <Slide direction="up" ref={ref} {...props} />;
});

export default function CarePlanDataViewer({ carePlan, careTeam }: { carePlan: CarePlan, careTeam: CareTeam }) {
    const [open, setOpen] = React.useState(false);

    const patient = getBgzData(carePlan, careTeam)

    const handleClickOpen = async () => {

        setOpen(true);

    };

    const handleClose = () => {
        setOpen(false);
    };

    if (!carePlan) return <></>

    return (
        <React.Fragment>
            <IconButton
                edge="start"
                color="inherit"
                onClick={handleClickOpen}
                aria-label="open"
            >
                <IconEyeSearch />

            </IconButton>
            <Dialog
                open={open}
                fullScreen
                TransitionComponent={Transition}
            >
                <AppBar sx={{ position: 'fixed', backgroundColor: '#121212' }}>
                    <Toolbar>
                        <IconButton
                            edge="start"
                            color="inherit"
                            onClick={handleClose}
                            aria-label="close"
                        >
                            <CloseIcon />
                        </IconButton>
                        <Typography sx={{ ml: 2, flex: 1 }} variant="h6" component="div">
                            Medical Records Viewer for BSN {getBsn(carePlan)} and CarePlan/{carePlan.id}
                        </Typography>
                    </Toolbar>
                </AppBar>
                <Box sx={{ mt: '80px' }}>

                </Box>
            </Dialog>
        </React.Fragment>
    );
}