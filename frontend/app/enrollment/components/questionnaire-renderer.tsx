'use client'
import { getResponse, SmartFormsRenderer, useQuestionnaireResponseStore } from '@aehrc/smart-forms-renderer';
import type { BundleEntry, FhirResource, Questionnaire, QuestionnaireResponse, Task } from 'fhir/r4';
import { useEffect, useState } from 'react';
import { Button } from '@/components/ui/button';
import { ReloadIcon } from "@radix-ui/react-icons";
import useEhrFhirClient from '@/hooks/use-ehr-fhir-client';
import { toast } from 'sonner';
import useCpsClient from '@/hooks/use-cps-client';
import useTaskProgressStore from '@/lib/store/task-progress-store';
import { findQuestionnaireResponse } from '@/lib/fhirUtils';
import { Spinner } from '@/components/spinner';
import { useStepper } from '@/components/stepper';
import { v4 } from 'uuid';
import { populateQuestionnaire } from '../../utils/populate';
import useEnrollmentStore from '@/lib/store/enrollment-store';

interface QuestionnaireRendererPageProps {
  questionnaire: Questionnaire;
  inputTask?: Task
}

const LoadingOverlay = () => (
  <div className="fixed inset-0 flex items-center justify-center bg-black bg-opacity-50 z-50">
    <div className="flex items-center justify-center space-x-2 text-white">
      <ReloadIcon className="mr-2 h-8 w-8 animate-spin" />
      <span>Submitting...</span>
    </div>
  </div>
);

function QuestionnaireRenderer(props: QuestionnaireRendererPageProps) {
  const { inputTask, questionnaire } = props;
  const updatableResponse = useQuestionnaireResponseStore.use.updatableResponse();

  const { patient, practitioner } = useEnrollmentStore()
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [prePopulated, setPrePopulated] = useState(false);
  const [initialized, setInitialized] = useState(false)
  const [prevQuestionnaireResponse, setPrevQuestionnaireReaspone] = useState<QuestionnaireResponse>()
  const { nextStep } = useStepper()

  const ehrClient = useEhrFhirClient()
  const cpsClient = useCpsClient()
  const { setTask } = useTaskProgressStore()

  useEffect(() => {

    const fetchQuestionnaireResponse = async () => {

      if (!inputTask || !questionnaire) return

      const questionnaireResponse = await findQuestionnaireResponse(inputTask, questionnaire) as QuestionnaireResponse

      console.log(`Found QuestionnaireResponse: ${JSON.stringify(questionnaireResponse)}`)

      setPrevQuestionnaireReaspone(questionnaireResponse)
    }

    if (questionnaire && !initialized) {

      console.log(`Fetching QuestionnaireResponse for Task/${inputTask?.id}`)
      fetchQuestionnaireResponse()

      setInitialized(true)
    }

  }, [initialized, inputTask, questionnaire])

  const submitQuestionnaireResponse = async () => {

    if (!inputTask || !updatableResponse) {
      toast.error("Cannot set QuestionnaireResponse, no Task provided")
      return
    }

    const outputTask = { ...inputTask }
    const questionnaireResponse = await findQuestionnaireResponse(inputTask, questionnaire)

    const newId = v4()
    updatableResponse.id = questionnaireResponse?.id ?? newId

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
        "reference": `QuestionnaireResponse/${updatableResponse.id}`,
        "type": "QuestionnaireResponse"
      }
    })

    outputTask.status = "completed"

    const bundle: FhirResource & { type: "transaction" } = {
      resourceType: 'Bundle',
      type: 'transaction',
      entry: [
        {
          fullUrl: outputTask.id,
          resource: {
            ...outputTask
          },
          request: {
            method: 'PUT',
            url: `Task/${outputTask.id}`
          }
        },
        {
          fullUrl: questionnaireResponse?.id ? updatableResponse.id : `urn:uuid:${updatableResponse.id}`,
          resource: {
            ...updatableResponse
          },
          request: {
            method: 'PUT',
            url: `QuestionnaireResponse/${updatableResponse.id}`
          }
        }
      ]
    };

    const resultBundle = await cpsClient?.transaction({
      body: bundle
    });

    nextStep()
  }

  useEffect(() => {
    if (!initialized || prePopulated || !patient || !practitioner) return

    const prePopulate = async () => {
      const { populateResult } = await populateQuestionnaire(questionnaire, patient, practitioner, {
        clientEndpoint: `http://localhost:9090/fhir`, //TODO: Fixme - not used as $populate is local
        authToken: null
      });

      if (populateResult && populateResult?.populated) {
        useQuestionnaireResponseStore.setState({ updatableResponse: populateResult.populated })
      }
    }

    if (!prePopulated) {
      prePopulate()
      setPrePopulated(true)
    }

  }, [initialized, patient, practitioner, questionnaire, prePopulated])

  if (!questionnaire) {
    return <div>Loading...</div>
  }

  return (
    <div className="margin-y">
      {isSubmitting && <LoadingOverlay />}

      <SmartFormsRenderer
        terminologyServerUrl={`${window.location.origin}${process.env.NEXT_PUBLIC_BASE_PATH ?? "/" + process.env.NEXT_PUBLIC_BASE_PATH}/api/terminology`}
        questionnaire={questionnaire}
        questionnaireResponse={prevQuestionnaireResponse}
      />
      <Button size="sm" disabled={isSubmitting} onClick={submitQuestionnaireResponse} className="float-right">
        {isSubmitting ? <Spinner /> : 'Next'}
      </Button>
    </div>
  );
}

export default QuestionnaireRenderer;
