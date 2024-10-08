/*
 * Copyright 2024 Commonwealth Scientific and Industrial Research
 * Organisation (CSIRO) ABN 41 687 119 230.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */
'use client'
import { populateQuestionnaire } from '../../utils/populate';
import type { Questionnaire, QuestionnaireResponse } from 'fhir/r4';
import { useEffect, useState } from 'react';
import useEnrollmentStore from '../../../lib/store/enrollment-store';
import { Button } from '@/components/ui/button';
import { ReloadIcon } from '@radix-ui/react-icons';
import { useQuestionnaireResponseStore } from '@aehrc/smart-forms-renderer';

interface PrePopButtonProps {
  questionnaire: Questionnaire;
  autoPopulate?: boolean
}

function PrePopButton(props: PrePopButtonProps) {
  const { questionnaire, autoPopulate } =
    props;

  const [isPopulating, setIsPopulating] = useState(false);
  const [autoPrePopulated, setAutoPrePopulated] = useState(false)
  const { patient, practitioner } = useEnrollmentStore()

  useEffect(() => {

    if (patient && practitioner && autoPopulate && !autoPrePopulated) {
      handlePrepopulate()
    }
  }, [patient, practitioner, autoPopulate, autoPrePopulated])

  async function handlePrepopulate() {
    if (!patient || !practitioner) {
      return;
    }
    setIsPopulating(true);

    const { populateResult } = await populateQuestionnaire(questionnaire, patient, practitioner, {
      clientEndpoint: `http://localhost:9090/fhir`, //TODO: Fixme - not used as $populate is local
      authToken: null
    });

    if (populateResult && populateResult?.populated) {
      useQuestionnaireResponseStore.setState({ updatableResponse: populateResult.populated })
      setAutoPrePopulated(true)
    }
    setIsPopulating(false);
  }

  if (!patient || !practitioner) {
    return <Button className="ml-[24px] mb-3" disabled>Pre-populate</Button>
  }

  if (autoPopulate) return <></>

  return (
    <Button
      className="ml-[24px] mb-5"
      onClick={handlePrepopulate}
      disabled={isPopulating}>
      {isPopulating ? <><ReloadIcon className="mr-2 h-4 w-4 animate-spin" /> Pre-populating</> : <>Pre-populate</>}
    </Button>
  );
}

export default PrePopButton;
