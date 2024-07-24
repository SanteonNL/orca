import Renderer from "./components/Renderer";
import type { Questionnaire } from 'fhir/r4';
import { headers } from 'next/headers'

export default async function Home() {
  //switching to dynamic rendering https://nextjs.org/docs/app/building-your-application/rendering/server-components#switching-to-dynamic-rendering
  const headersList = headers()

  if (!process.env.FHIR_BASE_URL) return <>FHIR Server not configured</>

  const resp = await fetch(`${process.env.FHIR_BASE_URL}/Questionnaire/2.16.840.1.113883.2.4.3.11.60.909.26.18--20240704100750`);

  if (!resp.ok) {
    const errorText = await resp.text();
    console.error('Failed to fetch Questionnaire:', resp.status, errorText);
    return <div>Failed to fetch Questionnaire: {resp.status} {errorText}</div>;
  }

  const questionnaire = await resp.json() as Questionnaire;

  return (
    <main className="flex min-h-screen flex-col items-center justify-between py-5">
      <Renderer questionnaire={questionnaire} bearerToken={null} />
    </main>
  );
}
