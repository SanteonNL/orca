import Client from 'fhir-kit-client';
import { Bundle, CarePlan, CarePlanActivity, Condition, Patient, Resource, ServiceRequest, Task } from 'fhir/r4';

type FhirClient = Client;
type FhirBundle<T extends Resource> = Bundle<T>;

const BSN_SYSTEM = "http://fhir.nl/fhir/NamingSystem/bsn"

const fetchAllBundlePages = async <T extends Resource>(
    client: FhirClient,
    initialBundle: FhirBundle<T>
): Promise<T[]> => {
    let allResources: T[] = [];
    let nextPageUrl: string | undefined = initialBundle.link?.find(link => link.relation === 'next')?.url;

    const processBundle = (bundle: FhirBundle<T>) => {
        if (bundle.entry) {
            const resources = bundle.entry.map(entry => entry.resource as T);
            allResources = allResources.concat(resources);
        }
    };

    processBundle(initialBundle);

    while (nextPageUrl) {
        const result = await client.nextPage({ bundle: { resourceType: 'Bundle', type: 'searchset', link: [{ relation: 'next', url: nextPageUrl }] } });
        const bundle = result as FhirBundle<T>;
        processBundle(bundle);
        nextPageUrl = bundle.link?.find(link => link.relation === 'next')?.url;
    }

    return allResources;
}

const getBsn = (patient?: Patient) => {
    return patient?.identifier?.find((identifier) => identifier.system === BSN_SYSTEM)?.value;
}

const getCarePlan = (patient: Patient, primaryCondition: Condition, relevantConditions?: Condition[]): CarePlan => {
    return {
        resourceType: 'CarePlan',
        status: 'active',
        intent: 'plan',
        subject: {
            identifier: {
                system: BSN_SYSTEM,
                value: getBsn(patient)
            }
        },
        addresses: [
            ...(primaryCondition ? [{
                identifier: {
                    system: primaryCondition?.code?.coding?.[0].system,
                    value: primaryCondition?.code?.coding?.[0].code,
                },
                display: primaryCondition?.code?.coding?.[0].display
            }] : []),
            ...(relevantConditions ? relevantConditions.map(condition => ({
                identifier: {
                    system: condition?.code?.coding?.[0].system,
                    value: condition?.code?.coding?.[0].code,
                },
                display: condition?.code?.coding?.[0].display
            })) : [])
        ],
        title: `Care Plan [${primaryCondition?.code?.text || ""}]`,
        description: "Care plan description here"
    }
}

const getTask = (carePlan: CarePlan, serviceRequest: ServiceRequest, primaryCondition: Condition): Task => {

    const conditionCode = primaryCondition.code?.coding?.[0]
    if (!conditionCode) throw new Error("Primary condition has no coding, cannot create Task")

    return {
        "resourceType": "Task",
        "basedOn": [
            {
                "reference": `CarePlan/${carePlan.id}`,
                type: "CarePlan"
            }
        ],
        "status": "requested",
        "intent": "order",
        focus: {
            identifier: {
                "system": conditionCode.system,
                "value": conditionCode.code
            },
            type: 'Condition'
        },
        input: [
            {
                type: {
                    coding: [{
                        system: "http://terminology.hl7.org/CodeSystem/task-input-type",
                        code: "Reference",
                        display: "Reference"
                    }]
                },
                valueReference: {
                    type: "ServiceRequest",
                    reference: '#contained-sr'
                }
            }
        ],
        contained: [
            {
                ...serviceRequest,
                id: 'contained-sr'
            }
        ]
    }
}

export { fetchAllBundlePages, getBsn, getCarePlan, getTask, BSN_SYSTEM }
