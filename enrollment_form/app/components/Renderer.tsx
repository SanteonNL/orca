'use client'
import { SmartFormsRenderer, getResponse, useQuestionnaireResponseStore } from '@aehrc/smart-forms-renderer';
import type { Questionnaire, QuestionnaireResponse } from 'fhir/r4';
import { useEffect, useState } from 'react';
import useEnrollmentStore from '../store/enrollmentStore';
import SelectedPatientView from './SelectedPatientView';
import SelectedServiceRequestView from './SelectedServiceRequestView';
import { Button } from '@/components/ui/button';
import { ReloadIcon } from "@radix-ui/react-icons";

interface RendererPageProps {
  questionnaire: Questionnaire;
  bearerToken: string | null;
}

const LoadingOverlay = () => (
  <div className="fixed inset-0 flex items-center justify-center bg-black bg-opacity-50 z-50">
    <div className="flex items-center justify-center space-x-2 text-white">
      <ReloadIcon className="mr-2 h-8 w-8 animate-spin" />
      <span>Submitting...</span>
    </div>
  </div>
);

function Renderer(props: RendererPageProps) {
  const { questionnaire } = props;
  const [questionnaireResponse, setQuestionnaireResponse] = useState<QuestionnaireResponse | null>(null);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const { patient, fetchPatient } = useEnrollmentStore();

  useEffect(() => {
    if (!patient) fetchPatient();
  }, [patient, fetchPatient]);

  const isValid = useQuestionnaireResponseStore.use.responseIsValid();
  const updatableResponse = useQuestionnaireResponseStore.use.updatableResponse();
  const invalidItems = useQuestionnaireResponseStore.use.invalidItems();

  useEffect(() => {
    console.log(`resp changed: ${JSON.stringify(updatableResponse, undefined, 2)} - isValid: ${isValid}`);
    console.log(`invalid items: ${JSON.stringify(invalidItems, undefined, 2)} - isValid: ${isValid}`);
  }, [updatableResponse, isValid, invalidItems]);

  const handleSubmit = (e: React.FormEvent<HTMLFormElement>) => {
    // if (!isValid) {
    //   e.preventDefault();
    //   return;
    // }

    setIsSubmitting(true);
    const response = getResponse();
    setQuestionnaireResponse(response);
  };

  return (
    <div className="margin-y">
      {isSubmitting && <LoadingOverlay />}
      <SelectedPatientView />
      <SelectedServiceRequestView />
      <SmartFormsRenderer
        questionnaire={questionnaire}
        questionnaireResponse={questionnaireResponse ?? undefined}
      />
      <form action="/orca/contrib/confirm" method="post" onSubmit={handleSubmit}>
        <Button disabled={isSubmitting} type="submit" className="ml-[24px]">
          {isSubmitting ? <ReloadIcon className="mr-2 h-4 w-4 animate-spin" /> : 'Submit'}
        </Button>
      </form>
    </div>
  );
}

export default Renderer;
