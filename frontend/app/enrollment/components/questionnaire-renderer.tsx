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
import { BSN_SYSTEM, findQuestionnaireResponse } from '@/lib/fhirUtils';
import { Spinner } from '@/components/spinner';
import { useStepper } from '@/components/stepper';
import { v4 } from 'uuid';
import { populateQuestionnaire } from '../../utils/populate';
import useEnrollmentStore from '@/lib/store/enrollment-store';

interface QuestionnaireRendererPageProps {
  questionnaire: Questionnaire;
  inputTask?: Task
}

const scpSubTaskIdentifierSystem = "http://santeonnl.github.io/shared-care-planning/scp-identifier"

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
  const [shouldScroll, setShouldScroll] = useState(false)

  const [prevQuestionnaireResponse, setPrevQuestionnaireReaspone] = useState<QuestionnaireResponse>()
  const { activeStep, setStep } = useStepper()

  const cpsClient = useCpsClient()
  const { onSubTaskSubmit } = useTaskProgressStore()

  useEffect(() => {
    if (shouldScroll) {
      window.scrollTo({ top: 0, behavior: 'smooth' })
      setShouldScroll(false);
    }
  }, [shouldScroll]);

  useEffect(() => {

    const fetchQuestionnaireResponse = async () => {

      if (!inputTask || !questionnaire) return

      const questionnaireResponse = await findQuestionnaireResponse(inputTask, questionnaire) as QuestionnaireResponse

      console.log(`Found QuestionnaireResponse: ${JSON.stringify(questionnaireResponse)}`)

      if (questionnaireResponse) {
        setPrevQuestionnaireReaspone(questionnaireResponse)
      }
    }

    if (questionnaire && !initialized) {

      console.log(`Fetching QuestionnaireResponse for Task/${inputTask?.id}`)
      fetchQuestionnaireResponse()

      setInitialized(true)
    }

  }, [initialized, inputTask, questionnaire])

  useEffect(() => {
    const markTaskAsInProgressWhenRequested = async () => {

      if (cpsClient && inputTask?.status === "requested") {
        //accept the task, and mark it as in-progress after as it's loaded by the front-end
        const acceptedTask = await cpsClient.update({
          resourceType: 'Task',
          body: { ...inputTask, status: "accepted" },
        })

        if (!acceptedTask) {
          toast.error("Failed to update Task status to 'accepted'")
        }
        const inProgressTask = await cpsClient?.update({
          resourceType: 'Task',
          body: { ...acceptedTask, status: "in-progress" },
        })
        if (!inProgressTask) {
          toast.error("Failed to update Task status to 'in-progress'")
        }
      }
    }

    markTaskAsInProgressWhenRequested()

  }, [cpsClient, inputTask])

  const submitQuestionnaireResponse = async () => {

    if (!inputTask || !updatableResponse) {
      toast.error("Cannot set QuestionnaireResponse, no Task provided")
      return
    }

    setIsSubmitting(true)

    setShouldScroll(true)

    const outputTask = { ...inputTask }
    const questionnaireResponse = await findQuestionnaireResponse(inputTask, questionnaire)

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

    const patientIdentifier = patient?.identifier?.find(id => id.system === BSN_SYSTEM) || patient?.identifier?.[0]

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
          resource: { ...updatableResponse, subject: { identifier: patientIdentifier } },
          request: {
            method: 'PUT',
            url: responseExists ? questionnaireResponseRef : `QuestionnaireResponse?identifier=${encodeURIComponent(`${scpSubTaskIdentifierSystem}|${newId}`)}`
          }
        }
      ]
    };

    await cpsClient?.transaction({
      body: bundle
    });

    onSubTaskSubmit(() => setStep(activeStep + 1))
  }

  useEffect(() => {
    if (!initialized || prePopulated || !patient || !practitioner) return

    const prePopulate = async () => {
      const { populateResult } = await populateQuestionnaire(questionnaire, patient, practitioner, {
        clientEndpoint: `http://localhost:9090/fhir`, //TODO: Fixme - not used as $populate is local
        authToken: null
      });

      if (populateResult && populateResult?.populated) {
        setPrevQuestionnaireReaspone(populateResult.populated)
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
        {isSubmitting && <Spinner className='mr-1 text-white' />}Next
      </Button>
    </div>
  );
}

export default QuestionnaireRenderer;
