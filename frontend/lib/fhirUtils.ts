import Client from 'fhir-kit-client';
import { Bundle, CarePlan, CarePlanActivity, Condition, Patient, Resource, ServiceRequest, Task } from 'fhir/r4';

type FhirClient = Client;
type FhirBundle<T extends Resource> = Bundle<T>;

export const BSN_SYSTEM = "http://fhir.nl/fhir/NamingSystem/bsn"

export const createEhrClient = () => {
    const baseUrl = process.env.NODE_ENV === "production"
        ? `${typeof window !== 'undefined' ? window.location.origin : ''}/orca/contrib/ehr/fhir`
        : "http://localhost:9090/fhir";

    return new Client({ baseUrl });
};

export const createCpsClient = () => {
    const baseUrl = process.env.NODE_ENV === "production"
        ? `${typeof window !== 'undefined' ? window.location.origin : ''}/orca/contrib/cps/fhir`
        : "http://localhost:7090/fhir";

    return new Client({ baseUrl });
};

export const fetchAllBundlePages = async <T extends Resource>(
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

export const getBsn = (patient?: Patient) => {
    return patient?.identifier?.find((identifier) => identifier.system === BSN_SYSTEM)?.value;
}

export const getCarePlan = (patient: Patient, conditions: Condition[], carePlanName: string): CarePlan => {
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
        addresses: conditions.map(condition => {
            return {
                identifier: {
                    system: condition?.code?.coding?.[0].system,
                    value: condition?.code?.coding?.[0].code,
                },
                display: condition?.code?.coding?.[0].display
            }
        }),
        title: carePlanName,
        description: "Care plan description here"
    }
}

export const getTask = (carePlan: CarePlan, serviceRequest: ServiceRequest, primaryCondition: Condition): Task => {

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