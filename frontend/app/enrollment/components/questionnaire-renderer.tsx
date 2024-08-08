'use client'
import { getResponse, SmartFormsRenderer, useQuestionnaireResponseStore } from '@aehrc/smart-forms-renderer';
import type { Questionnaire, QuestionnaireResponse, Task } from 'fhir/r4';
import { useEffect, useState } from 'react';
import SelectedPatientView from './selected-patient-view';
import SelectedServiceRequestView from './selected-service-request-view';
import { Button } from '@/components/ui/button';
import { ReloadIcon } from "@radix-ui/react-icons";
import PrePopButton from './pre-pop-button';
import useEhrFhirClient from '@/hooks/use-ehr-fhir-client';
import { toast } from 'sonner';
import useCpsClient from '@/hooks/use-cps-client';
import useTaskProgressStore from '@/lib/store/task-progress-store';
import { findQuestionnaireResponse, findQuestionnaire, getQuestionnaireResponseId } from '@/lib/fhirUtils';
import { Spinner } from '@/components/spinner';
import { useStepper } from '@/components/stepper';

interface QuestionnaireRendererPageProps {
  questionnaire?: Questionnaire;
  inputTask?: Task
  onSubmit?(): void
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
  const { onSubmit, inputTask } = props;
  // const [questionnaireResponse, setQuestionnaireResponse] = useState<QuestionnaireResponse | null>(null);
  // const questionnaireResponse = useQuestionnaireResponseStore.use.sourceResponse()
  const updatableResponse = useQuestionnaireResponseStore.use.updatableResponse();

  const [isSubmitting, setIsSubmitting] = useState(false);
  const [questionnaire, setQuestionnaire] = useState<Questionnaire>()
  const [initialized, setInitialized] = useState(false)
  const [prevQuestionnaireResponse, setPrevQuestionnaireReaspone] = useState<QuestionnaireResponse>()
  const { nextStep } = useStepper()

  const ehrClient = useEhrFhirClient()
  const cpsClient = useCpsClient()
  const { setTask } = useTaskProgressStore()

  useEffect(() => {

    if (questionnaire && !initialized) {

      setPrevQuestionnaireReaspone(
        findQuestionnaireResponse(inputTask, questionnaire)
      )

      setInitialized(true)
    }

  }, [initialized, inputTask, questionnaire])


  useEffect(() => {

    if (inputTask) {
      setQuestionnaire(
        findQuestionnaire(inputTask)
      )
    }
  }, [inputTask])

  const submitQuestionnaireResponse = async () => {

    if (!inputTask || !updatableResponse) {
      toast.error("Cannot set QuestionnaireResponse, no Task provided")
      return
    }

    const outputTask = { ...inputTask }
    const questionnaireResponseId = getQuestionnaireResponseId(questionnaire)
    updatableResponse.id = questionnaireResponseId
    outputTask.contained?.push(updatableResponse)

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
        "reference": questionnaireResponseId,
        "type": "QuestionnaireResponse"
      }
    })

    outputTask.status = "accepted" //TODO: This should be done by the CPS itself based on order management

    const resultTask = await cpsClient?.update({
      resourceType: "Task",
      id: outputTask.id,
      body: outputTask
    })

    setTask(resultTask as Task)

    nextStep()
  }

  if (!questionnaire) {
    return <div>Loading...</div>
  }

  // const isValid = useQuestionnaireResponseStore.use.responseIsValid();
  // const updatableResponse = useQuestionnaireResponseStore.use.updatableResponse();
  // const invalidItems = useQuestionnaireResponseStore.use.invalidItems();

  const handleSubmit = (e: React.FormEvent<HTMLFormElement>) => {
    // if (!isValid) {
    //   e.preventDefault();
    //   return;
    // }

    setIsSubmitting(true);
    const response = getResponse();
    // setQuestionnaireResponse(response);

    if (onSubmit) onSubmit()
  };

  return (
    <div className="margin-y">
      {isSubmitting && <LoadingOverlay />}
      <SelectedPatientView />
      <SelectedServiceRequestView />
      <PrePopButton
        autoPopulate
        questionnaire={questionnaire}
      />
      <SmartFormsRenderer
        terminologyServerUrl={`${window.location.origin}${process.env.NEXT_PUBLIC_BASE_PATH ?? "/" + process.env.NEXT_PUBLIC_BASE_PATH}/api/terminology`}
        questionnaire={questionnaire}
        questionnaireResponse={prevQuestionnaireResponse}
      />
      {/* TODO: Update QuestionnaireResponse if it exists on Task.output */}
      <Button size="sm" disabled={isSubmitting} onClick={submitQuestionnaireResponse} className="float-right">
        {isSubmitting ? <Spinner /> : 'Next'}
      </Button>
    </div>
  );
}

export default QuestionnaireRenderer;
