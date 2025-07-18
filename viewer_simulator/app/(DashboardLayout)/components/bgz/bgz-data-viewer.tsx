import React from 'react'

import { IconEyeSearch } from '@tabler/icons-react';
import { CarePlan, CareTeam, Patient } from 'fhir/r4';
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
import BgzRecordsViewer from '../shared/fhir/BgzRecordsViewer';
import useBgzStore from '@/store/bgz-store';
import Loading from '@/app/loading';


const Transition = React.forwardRef(function Transition(
    props: TransitionProps & {
        children: React.ReactElement;
    },
    ref: React.Ref<unknown>,
) {
    return <Slide direction="up" ref={ref} {...props} />;
});

export default function BgzDataViewer({ name, carePlan }: { name: string, carePlan: CarePlan }) {
    const [open, setOpen] = React.useState(false);
    const { addBgzData, clearBgzData, loaded, setLoaded } = useBgzStore()

    const handleClickOpen = async () => {
        setOpen(true);
        clearBgzData()
        const bgzData = await getBgzData(name, carePlan)

        // console.log('bgzData', JSON.stringify(bgzData, null, 2))
        addBgzData(bgzData)
        setLoaded(true)
    };

    const handleClose = () => {
        setOpen(false);
    };

    if (!carePlan) return <></>

    return (
        <>
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
                            Medical Records Viewer for CarePlan/{carePlan.id}
                        </Typography>
                    </Toolbar>
                </AppBar>
                <Box sx={{ mt: '80px' }}>
                    {loaded ? <BgzRecordsViewer /> : <Loading />}
                </Box>
            </Dialog>
        </>
    );
}