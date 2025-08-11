'use client'

import { useQuestionnaireResponseStore, BaseRenderer, useBuildForm, useRendererQueryClient } from '@aehrc/smart-forms-renderer';
import type { FhirResource, Questionnaire, QuestionnaireResponse, Task } from 'fhir/r4';
import { useEffect, useState } from 'react';

import { toast } from 'sonner';
import { findQuestionnaireResponse, getPatientIdentifier } from '@/lib/fhirUtils';
import { Spinner } from '@/components/spinner';
import { v4 } from 'uuid';
import { populateQuestionnaire } from '../../utils/populate';
import useEnrollment from '@/app/hooks/enrollment-hook';
import { QueryClientProvider } from '@tanstack/react-query';
import { Button, createTheme, Shadows, ThemeProvider } from '@mui/material';
import Loading from '../loading';
import {useContextStore} from "@/lib/store/context-store";

interface QuestionnaireRendererPageProps {
  questionnaire: Questionnaire;
  inputTask?: Task
}

const scpSubTaskIdentifierSystem = "http://santeonnl.github.io/shared-care-planning/scp-identifier"

function QuestionnaireRenderer(props: QuestionnaireRendererPageProps) {
  const { inputTask, questionnaire } = props;
  const updatableResponse = useQuestionnaireResponseStore.use.updatableResponse();
  const responseIsValid = useQuestionnaireResponseStore.use.responseIsValid();

  const { launchContext, cpsClient } = useContextStore();
  const { patient, practitioner } = useEnrollment()
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [prePopulated, setPrePopulated] = useState(false);
  const [initialized, setInitialized] = useState(false)

  const [, setPrevQuestionnaireResponse] = useState<QuestionnaireResponse>()

  useEffect(() => {
    const fetchQuestionnaireResponse = async () => {

      if (!inputTask || !questionnaire || !launchContext || !cpsClient) return

      const questionnaireResponse = await findQuestionnaireResponse(cpsClient, inputTask, questionnaire) as QuestionnaireResponse

      if (questionnaireResponse) {
        setPrevQuestionnaireResponse(questionnaireResponse)
      }
    }

    if (questionnaire && !initialized) {

      console.log(`Fetching QuestionnaireResponse for Task/${inputTask?.id}`)
      fetchQuestionnaireResponse()

      setInitialized(true)
    }

  }, [initialized, inputTask, questionnaire, launchContext, cpsClient])

  const submitQuestionnaireResponse = async (e: React.MouseEvent<HTMLButtonElement, MouseEvent>) => {

    e.preventDefault()

    if (!inputTask || !updatableResponse) {
      toast.error("Cannot set QuestionnaireResponse, no Task provided")
      return
    }
    if (!launchContext || !cpsClient) {
      toast.error("No launch context found")
      return
    }

    setIsSubmitting(true)

    const outputTask = { ...inputTask }
    const questionnaireResponse = await findQuestionnaireResponse(cpsClient, inputTask, questionnaire)

    const newId = v4()
    const responseExists = !!questionnaireResponse?.id
    const questionnaireResponseRef = responseExists ? `QuestionnaireResponse/${questionnaireResponse.id}` : `urn:uuid:${newId}`

    if (!responseExists) {
      updatableResponse.identifier = {
        system: scpSubTaskIdentifierSystem,
        value: newId
      }
    }

    if (!outputTask.output) outputTask.output = []

    outputTask.output.push({
      "type": {
        "coding": [{
          "system": "http://terminology.hl7.org/CodeSystem/task-input-type",
          "code": "Reference",
          "display": "Reference"
        }]
      },
      "valueReference": {
        "reference": questionnaireResponseRef,
        "type": "QuestionnaireResponse"
      }
    })

    outputTask.status = "completed"

    const bundle: FhirResource & { type: "transaction" } = {
      resourceType: 'Bundle',
      type: 'transaction',
      entry: [
        {
          fullUrl: `Task/${outputTask.id}`,
          resource: {
            ...outputTask
          },
          request: {
            method: 'PUT',
            url: `Task/${outputTask.id}`
          }
        },
        {
          fullUrl: questionnaireResponseRef,
          resource: { ...updatableResponse, subject: { identifier: getPatientIdentifier(patient) } },
          request: {
            method: 'PUT',
            url: responseExists ? questionnaireResponseRef : `QuestionnaireResponse?identifier=${encodeURIComponent(`${scpSubTaskIdentifierSystem}|${newId}`)}`
          }
        }
      ]
    };

    await cpsClient!.transaction({
      body: bundle
    });


  }

  useEffect(() => {
    if (!initialized || prePopulated || !patient || !practitioner) return

    const prePopulate = async () => {
      const { populateResult } = await populateQuestionnaire(questionnaire, patient, practitioner, {
        clientEndpoint: `http://localhost:9090/fhir`, //TODO: Fixme - not used as $populate is local
        authToken: null
      });

      if (populateResult && populateResult?.populated) {
        setPrevQuestionnaireResponse(populateResult.populated)
        // updatableResponse = populateResult.populated
        useQuestionnaireResponseStore.setState({ updatableResponse: populateResult.populated })
      }
    }

    if (!prePopulated) {
      console.log('prePopulating')
      prePopulate()
      setPrePopulated(true)
    }

  }, [initialized, patient, practitioner, questionnaire, prePopulated])

  const queryClient = useRendererQueryClient();

  // This hook builds the form based on the questionnaire
  const isBuilding = useBuildForm(questionnaire);

  const theme = createTheme({
    palette: {
      primary: {
        main: '#1c6268',
      },
    },
    shadows: Array(25).fill('none') as Shadows,
    components: {
      MuiGrid: {
        defaultProps: {
          mb: 1
        },
      },
      MuiCard: {
        styleOverrides: {
          root: {
            backgroundColor: '#F5F5F5',
          },
        },
      }
    }
  });

  if (!questionnaire || isBuilding) {
    return <Loading />
  }

  return (
    <div className="margin-y">
      <ThemeProvider theme={theme}>
        <QueryClientProvider client={queryClient}>
          <BaseRenderer />
        </QueryClientProvider>

        <div className='flex gap-3 mt-5'>
          <Button type="button" variant='contained' disabled={isSubmitting || !responseIsValid} onClick={submitQuestionnaireResponse}>
            {isSubmitting && <Spinner className='h-6 mr-2 text-inherit' />}
            Verzoek versturen
          </Button>
        </div>
      </ThemeProvider>
    </div>
  );
}

export default QuestionnaireRenderer;
