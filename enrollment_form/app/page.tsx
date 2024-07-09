import Renderer from "./components/Renderer";
import type { Questionnaire } from 'fhir/r4';

export default async function Home() {

  const questionnaireResp = await fetch(`${process.env.FHIR_BASE_URL}/Questionnaire/2.16.840.1.113883.2.4.3.11.60.909.26.18--20240704100750`)
  const questionnaire = await questionnaireResp.json() as Questionnaire

  return (
    <main className="flex min-h-screen flex-col items-center justify-between py-5">
      <Renderer questionnaire={questionnaire} bearerToken={null} />
    </main>
  );
}
