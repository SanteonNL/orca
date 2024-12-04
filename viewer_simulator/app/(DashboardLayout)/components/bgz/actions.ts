"use server"
import { Bundle, CarePlan, CareTeam } from "fhir/r4";

export async function getBgzData(carePlan: CarePlan, careTeam: CareTeam) {

    //TODO: Use a Promise.all for all of the bgz requests

    console.log(`Sending request to ${process.env.FHIR_AGGREGATE_URL}/Patient?_include=Patient:general-practitioner with X-Scp-Context: ${process.env.ORCA_CAREPLANCONTRIBUTOR_CAREPLANSERVICE_URL}/CarePlan/${carePlan.id}`);

    const response = await fetch(`${process.env.FHIR_AGGREGATE_URL}/Patient?_include=Patient:general-practitioner`, {
        headers: {
            "Authorization": `Bearer ${process.env.FHIR_AUTHORIZATION_TOKEN}`,
            "X-Scp-Context": `${process.env.ORCA_CAREPLANCONTRIBUTOR_CAREPLANSERVICE_URL}/CarePlan/${carePlan.id}`
        },
    });

    if (!response.ok) {
        console.error(`Failed to fetch Patient: ${response.statusText}`);
        return;
    }

    const result = await response.json() as Bundle

    // const conditionResponse = await fetch(`${process.env.FHIR_AGGREGATE_URL}/Condition`, {
    //     headers: {
    //         "Authorization": `Bearer ${process.env.FHIR_AUTHORIZATION_TOKEN}`,
    //         "X-Scp-Context": `${process.env.ORCA_CAREPLANCONTRIBUTOR_CAREPLANSERVICE_URL}/CarePlan/${carePlan.id}`
    //     },
    // });

    // if (!conditionResponse.ok) {
    //     console.error(`Failed to fetch Condition: ${conditionResponse.statusText}`);
    //     return;
    // }

    // const conditionResult = await conditionResponse.json() as Bundle

    // const combinedBundle: Bundle = {
    //     resourceType: "Bundle",
    //     type: "collection",
    //     entry: [
    //         ...(result.entry || []),
    //         ...(conditionResult.entry || [])
    //     ]
    // };

    // console.log(JSON.stringify(combinedBundle));

    // return combinedBundle;

    console.log(JSON.stringify(result));

    return result;
}