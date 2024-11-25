import React, { useState } from 'react'

import { SmartFormsRenderer } from '@aehrc/smart-forms-renderer';
import { IconCheckupList } from '@tabler/icons-react';
import { Questionnaire, QuestionnaireResponse, Task } from 'fhir/r4';
import { fetchQuestionnaire, fetchQuestionnaireResponse } from '@/utils/fhirUtils';
import Dialog from '@mui/material/Dialog';
import AppBar from '@mui/material/AppBar';
import Toolbar from '@mui/material/Toolbar';
import Typography from '@mui/material/Typography';
import CloseIcon from '@mui/icons-material/Close';
import Slide from '@mui/material/Slide';
import { TransitionProps } from '@mui/material/transitions';
import { Box, IconButton } from '@mui/material';
import Loading from '@/app/loading';

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
    const [questionnaire, setQuestionnaire] = useState<Questionnaire>()
    const [questionnaireResponse, setQuestionnaireResponse] = useState<QuestionnaireResponse>()
    const [fetched, setFetched] = useState(false)

    const handleClickOpen = async () => {

        setOpen(true);

        if (!fetched) {
            const questionnaire = await fetchQuestionnaire(task)
            setQuestionnaire(questionnaire)
            setQuestionnaireResponse(await fetchQuestionnaireResponse(task, questionnaire))
            setFetched(true)
        }
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
                    {!fetched ?
                        (<Loading />) :
                        !questionnaire || !questionnaireResponse ? (
                            <>No Questionnaire or QuestionnaireResponse found</>
                        ) : (
                            <SmartFormsRenderer
                                readOnly
                                questionnaire={questionnaire}
                                questionnaireResponse={questionnaireResponse}
                            />
                        )
                    }

                </Box>
            </Dialog>
        </React.Fragment>
    );
}