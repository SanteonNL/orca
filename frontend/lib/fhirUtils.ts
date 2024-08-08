import Client from 'fhir-kit-client';
import { Bundle, CarePlan, CarePlanActivity, Condition, Patient, Questionnaire, QuestionnaireResponse, Resource, ServiceRequest, Task } from 'fhir/r4';

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

export const getTask = (carePlan: CarePlan, serviceRequest: ServiceRequest, primaryCondition: Condition, questionnaire: Questionnaire): Task => {

    const conditionCode = primaryCondition.code?.coding?.[0]
    if (!conditionCode) throw new Error("Primary condition has no coding, cannot create Task")

    return {
        "resourceType": "Task",
        "basedOn": [
            {
                "reference": `CarePlan/${carePlan.id}`,
                type: "CarePlan",
                display: carePlan.title
            }
        ],
        for: carePlan.subject,
        "status": "requested",
        "intent": "order",
        focus: {
            identifier: {
                "system": conditionCode.system,
                "value": conditionCode.code
            },
            display: conditionCode.display,
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
            },
            {
                type: {
                    coding: [{
                        system: "http://terminology.hl7.org/CodeSystem/task-input-type",
                        code: "Reference",
                        display: "Reference"
                    }]
                },
                valueReference: {
                    type: "Questionnaire",
                    reference: '#questionnaire'
                }
            },
        ],
        contained: [
            {
                ...serviceRequest,
                id: 'contained-sr'
            },
            {
                ...questionnaire,
                id: 'questionnaire'
            }
        ]
    }
}

export const getTaskPerformer = (task?: Task) => {

    if (!task) return undefined

    const serviceRequestFromInput = task.contained?.find((contained) => {
        return contained.resourceType === "ServiceRequest"
    })

    return serviceRequestFromInput?.performer?.[0]
}

export const getQuestionnaireResponseId = (questionnaire?: Questionnaire) => {
    if (!questionnaire) throw new Error("Tried to generate a questionnaire response id but the Questionnaire is not defined")
    return `#questionnaire-response-${questionnaire.id}`
}

export const findQuestionnaire = (task?: Task) => {
    if (!task || !task.contained) return

    const questionnaires = task.contained.filter((contained) => contained.resourceType === "Questionnaire") as Questionnaire[]

    if (questionnaires.length < 1) console.warn("Found more than one Questionnaire for Task/" + task.id)

    return questionnaires.length ? questionnaires[0] : undefined
}

export const findQuestionnaireResponse = (task?: Task, questionnaire?: Questionnaire) => {
    if (!task || !task.contained || !questionnaire) return

    const expectedQuestionnaireId = getQuestionnaireResponseId(questionnaire)
    return task.contained.find((contained) => contained.id === expectedQuestionnaireId) as QuestionnaireResponse | undefined
}