'use client'
import { SmartFormsRenderer, getResponse, useQuestionnaireResponseStore } from '@aehrc/smart-forms-renderer';
import type { Patient, Practitioner, Questionnaire, QuestionnaireResponse, QuestionnaireItem, QuestionnaireResponseItem } from 'fhir/r4';
import { useEffect, useState } from 'react';

interface RendererPageProps {
  questionnaire: Questionnaire;
  bearerToken: string | null;
}

function Renderer(props: RendererPageProps) {
  const { questionnaire, bearerToken } = props;
  const [questionnaireResponse, setQuestionnaireResponse] = useState<QuestionnaireResponse | null>(null);
  //TODO: Fetch the resources from /orca/contrib/patient etc and add the <PrePopButton>
  const [patient, setPatient] = useState<Patient | null>(null);
  const [practitioner, setPractitioner] = useState<Practitioner | null>(null);

  const isValid = useQuestionnaireResponseStore.use.responseIsValid();
  const updatableResponse = useQuestionnaireResponseStore.use.updatableResponse();
  const invalidItems = useQuestionnaireResponseStore.use.invalidItems();

  useEffect(() => {
    console.log(`resp changed: ${JSON.stringify(updatableResponse, undefined, 2)} - isValid: ${isValid}`)
    console.log(`invalid items: ${JSON.stringify(invalidItems, undefined, 2)} - isValid: ${isValid}`)
  }, [updatableResponse, isValid, invalidItems])

  const handleSubmit = () => {
    if (!isValid) return

    const response = getResponse();
    setQuestionnaireResponse(response);
  };

  return (
    <div className="margin-y">
      <SmartFormsRenderer
        questionnaire={questionnaire}
        questionnaireResponse={questionnaireResponse ?? undefined}
      />
      <button disabled={!isValid} className='rounded-lg bg-blue-500 w-[calc(100%-48px)] ml-[24px] p-4 text-white hover:bg-blue-400 disabled:bg-blue-200' onClick={handleSubmit}>
        Submit
      </button>
    </div>
  )
}

export default Renderer;
