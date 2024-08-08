import React from 'react'

import { SmartFormsRenderer } from '@aehrc/smart-forms-renderer';
import { IconCheckupList } from '@tabler/icons-react';
import { Task } from 'fhir/r4';
import { findQuestionnaire, findQuestionnaireResponse } from '@/utils/fhirUtils';
import Dialog from '@mui/material/Dialog';
import AppBar from '@mui/material/AppBar';
import Toolbar from '@mui/material/Toolbar';
import Typography from '@mui/material/Typography';
import CloseIcon from '@mui/icons-material/Close';
import Slide from '@mui/material/Slide';
import { TransitionProps } from '@mui/material/transitions';
import { Box, IconButton } from '@mui/material';

const Transition = React.forwardRef(function Transition(
    props: TransitionProps & {
        children: React.ReactElement;
    },
    ref: React.Ref<unknown>,
) {
    return <Slide direction="up" ref={ref} {...props} />;
});

export default function ViewTaskOutput({ task }: { task: Task }) {
    const [open, setOpen] = React.useState(false);

    const questionnaire = findQuestionnaire(task)
    const questionnaireResponse = findQuestionnaireResponse(task, questionnaire)

    const handleClickOpen = () => {
        setOpen(true);
    };

    const handleClose = () => {
        setOpen(false);
    };

    return (
        <React.Fragment>
            <IconButton
                edge="start"
                color="inherit"
                onClick={handleClickOpen}
                aria-label="open"
            >
                <IconCheckupList />

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
                            QuestionnaireResponse for {questionnaire?.title || questionnaire?.id || "unknown"}
                        </Typography>
                    </Toolbar>
                </AppBar>
                <Box sx={{ mt: '80px' }}>
                    {!questionnaire || !questionnaireResponse ? (
                        <>No Questionnaire or QuestionnaireResponse found</>
                    ) : (
                        <SmartFormsRenderer
                            readOnly
                            questionnaire={questionnaire}
                            questionnaireResponse={questionnaireResponse}
                        />
                    )}
                </Box>
            </Dialog>
        </React.Fragment>
    );
}