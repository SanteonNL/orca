import React, { useState } from 'react'

import { SmartFormsRenderer } from '@aehrc/smart-forms-renderer';
import { IconCheckupList } from '@tabler/icons-react';
import {BundleEntry, FhirResource, Questionnaire, QuestionnaireResponse, Task} from 'fhir/r4';
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

interface ViewTaskOutputProps {
    task: Task;
    notificationBundles: BundleEntry<FhirResource>[];
}

export default function ViewTaskOutput({ task, notificationBundles }: ViewTaskOutputProps) {
    const [open, setOpen] = React.useState(false);
    const [questionnaire, setQuestionnaire] = useState<Questionnaire>()
    const [questionnaireResponse, setQuestionnaireResponse] = useState<QuestionnaireResponse>()
    const [fetched, setFetched] = useState(false)

    const handleClickOpen = async () => {

        setOpen(true);

        if (!fetched) {
            if (task && task.input && task.output) {
                const questionnaireRefs = task.input
                    .filter((input) => input.valueReference?.reference?.startsWith("Questionnaire/"))
                    .map((input) => input.valueReference?.reference)

                if (questionnaireRefs.length > 1) console.warn("Found more than one Questionnaire for Task/" + task.id)

                let q = notificationBundles.filter((entry) => entry.resource?.resourceType === "Questionnaire")
                    .map((entry) => entry.resource as Questionnaire)
                    .find((questionnaire) => questionnaire.id === questionnaireRefs[0]?.replace("Questionnaire/", ""))

                setQuestionnaire(q)

                const questionnaireResponseRefs = task.output
                    .filter((output) => output.valueReference?.reference?.startsWith("QuestionnaireResponse/"))
                    .map((output) => output.valueReference?.reference)

                if (questionnaireResponseRefs.length > 1) console.warn("Found more than one QuestionnaireResponse for Task/" + task.id)

                let qr = notificationBundles.filter((entry) => entry.resource?.resourceType === "QuestionnaireResponse")
                    .map((entry) => entry.resource as QuestionnaireResponse)
                    .find((questionnaireResponse) => questionnaireResponse.id === questionnaireResponseRefs[0]?.replace("QuestionnaireResponse/", ""))

                setQuestionnaireResponse(qr)

            }
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