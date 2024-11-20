import Client from 'fhir-kit-client';
import { Bundle, CarePlan, Condition, Patient, Questionnaire, Reference, Resource, ServiceRequest, Task } from 'fhir/r4';

type FhirClient = Client;
type FhirBundle<T extends Resource> = Bundle<T>;

export const BSN_SYSTEM = "http://fhir.nl/fhir/NamingSystem/bsn"

export const createEhrClient = () => {
    const baseUrl = process.env.NODE_ENV === "production"
        ? `${typeof window !== 'undefined' ? window.location.origin : ''}/orca/cpc/ehr/fhir`
        : "http://localhost:9090/fhir";

    return new Client({ baseUrl });
};

export const createCpsClient = () => {
    const baseUrl = process.env.NODE_ENV === "production"
        ? `${typeof window !== 'undefined' ? window.location.origin : ''}/orca/cpc/cps/fhir`
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

export const findInBundle = (resourceType: string, bundle?: Bundle) => {
    return bundle?.entry?.find((entry) => entry.resource?.resourceType === resourceType)?.resource;
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

export const getTaskBundle = (serviceRequest: ServiceRequest, primaryCondition: Condition, patient: Patient, carePlanRef?: Reference): Bundle  & { type: "transaction" } => {
    return {
        resourceType: "Bundle",
        type: "transaction",
        entry: [
            {
                fullUrl: "urn:uuid:patient",
                resource: {...patient, id: undefined},
                request: {
                    method: "PUT",
                    url: `Patient?identifier=http://fhir.nl/fhir/NamingSystem/bsn|${getBsn(patient)}`
                }
            },
            {
                fullUrl: "urn:uuid:serviceRequest",
                //FIXME: Check if Patient is the same as in Bundle
                resource: {...serviceRequest, id: undefined, subject: {reference: "urn:uuid:patient"}},
                request: {
                    method: "POST",
                    url: "ServiceRequest"
                }
            },
            {
                fullUrl: "urn:uuid:task",
                resource: getBundleTask(serviceRequest, primaryCondition, carePlanRef),
                request: {
                    method: "POST",
                    url: "Task"
                }
            }
        ]
    }
}

export const getBundleTask = (serviceRequest: ServiceRequest, primaryCondition: Condition, carePlanRef?: Reference): Task => {
    const conditionCode = primaryCondition.code?.coding?.[0]
    if (!conditionCode) throw new Error("Primary condition has no coding, cannot create Task")

    //TODO: See if the ServiceRequest needs to be included in the Task via input or in a Bundle
    return {
        resourceType: "Task",
        meta: {
            profile: [
                "http://santeonnl.github.io/shared-care-planning/StructureDefinition/SCPTask"
            ]
        },

        //TODO: Not setting this, made by CPS - but should maybe set it for existing
        basedOn: carePlanRef && [carePlanRef],
        for: {
            reference: "urn:uuid:patient"
        },
        status: "requested",
        intent: "order",
        reasonCode: {
            coding: [conditionCode]
        },
        requester: {
            identifier: serviceRequest.requester?.identifier,
        },
        owner: {
            identifier: serviceRequest.performer?.[0]?.identifier,
        },
        focus: {
            display: serviceRequest.code?.coding?.[0].display,
            type: "ServiceRequest",
            reference: "urn:uuid:serviceRequest"
        },
    }
}

export const findQuestionnaireResponse = async (task?: Task, questionnaire?: Questionnaire) => {
    if (!task || !task.output || !questionnaire) return

    const questionnaireResponse = task.output.find((output) => {
        return output.valueReference?.reference?.startsWith(`QuestionnaireResponse/`)
    })

    if (!questionnaireResponse) return

    const questionnaireResponseId = questionnaireResponse.valueReference?.reference
    if (!questionnaireResponseId) return

    const cpsClient = createCpsClient()
    return await cpsClient.read({
        resourceType: "QuestionnaireResponse",
        id: questionnaireResponseId.split("/")[1]
    })
}

export function getPatientAddress(patient?: Patient) {
    const address = patient?.address?.[0]
    if (!address) return "Unknown address"

    return `${address.line?.join(", ")}, ${address.postalCode} ${address.city}`
}