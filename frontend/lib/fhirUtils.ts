import Client from 'fhir-kit-client';
import {
    Bundle, Coding,
    Condition,
    Identifier,
    Patient,
    PractitionerRole,
    Questionnaire,
    Reference,
    Resource,
    ServiceRequest,
    Task
} from 'fhir/r4';


type FhirClient = Client;
type FhirBundle<T extends Resource> = Bundle<T>;

export const patientIdentifierSystem = () => {
    return process.env.ORCA_PATIENT_IDENTIFIER_SYSTEM ?? "http://fhir.nl/fhir/NamingSystem/bsn";
}

// This function creates a FHIR client to communicate with other (remote) SCP nodes' FHIR APIs.
export const createScpClient = (tenantId: string) => {
    const baseUrl = `${typeof window !== 'undefined' ? window.location.origin : ''}/orca/cpc/${tenantId}/external/fhir`;
    return new Client({baseUrl});
};

// This function creates a FHIR client to communicate with the EHR's FHIR API.
export const createEhrClient = (tenantId: string) => {
    const baseUrl = `${typeof window !== 'undefined' ? window.location.origin : ''}/orca/cpc/${tenantId}/ehr/fhir`;
    return new Client({baseUrl});
};

// This function creates a FHIR client to communicate with the ORCA instance's own CarePlanService.
export const createCpsClient = (tenantId: string) => {
    const baseUrl = `${typeof window !== 'undefined' ? window.location.origin : ''}/orca/cpc/${tenantId}/external/fhir`;
    return new Client({
        baseUrl: baseUrl,
        customHeaders: {
            'X-Scp-Fhir-Url': 'local-cps',
        }
    });
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
        const result = await client.nextPage({
            bundle: {
                resourceType: 'Bundle',
                type: 'searchset',
                link: [{relation: 'next', url: nextPageUrl}]
            }
        });
        const bundle = result as FhirBundle<T>;
        processBundle(bundle);
        nextPageUrl = bundle.link?.find(link => link.relation === 'next')?.url;
    }

    return allResources;
}

export const getPatientIdentifier = (patient?: Patient) => {
    return patient?.identifier?.find((identifier) => identifier.system === patientIdentifierSystem());
}

export const findInBundle = (resourceType: string, bundle?: Bundle) => {
    return bundle?.entry?.find((entry) => entry.resource?.resourceType === resourceType)?.resource;
}

const cleanPatient = (patient: Patient) => {
    const cleanedPatient = {...patient, id: undefined}
    if (cleanedPatient.contact) {
        for (const contact of cleanedPatient.contact) {
            if (contact.organization?.reference) {
                delete contact.organization.reference;
            }
        }
    }

    if (cleanedPatient.managingOrganization?.reference) {
        delete cleanedPatient.managingOrganization.reference;
    }

    if (cleanedPatient.link) {
        for (const link of cleanedPatient.link) {
            if (link.other?.reference) {
                delete link.other.reference;
            }
        }
    }
    if (cleanedPatient.generalPractitioner) {
        for (const practitioner of cleanedPatient.generalPractitioner) {
            if (practitioner.reference) {
                delete practitioner.reference;
            }
        }
    }

    return cleanedPatient;
}

const cleanServiceRequest = (serviceRequest: ServiceRequest, patient: Patient, patientReference: string, taskIdentifier?: string) => {
    // Clean up the ServiceRequest by removing relative references - the CPS won't understand them
    const cleanedServiceRequest = {...serviceRequest, id: undefined};

    const patientIdentifier = getPatientIdentifier(patient);
    if (!patientIdentifier || serviceRequest.subject?.identifier?.system !== patientIdentifier.system || serviceRequest.subject?.identifier?.value !== patientIdentifier.value) {
        throw new Error("ServiceRequest.subject.identifier in service request differs from patient.identifier");
    }

    if (typeof cleanedServiceRequest.subject !== 'object') {
        cleanedServiceRequest.subject = {};
    }
    cleanedServiceRequest.subject.reference = patientReference;

    if (cleanedServiceRequest.requester?.reference) {
        delete cleanedServiceRequest.requester.reference;
    }

    for (const item of cleanedServiceRequest?.reasonReference || []) {
        if (item?.reference) {
            delete item.reference;
        }
    }

    for (const item of cleanedServiceRequest?.performer || []) {
        if (item?.reference) {
            delete item.reference;
        }
    }

    if (taskIdentifier) {
        cleanedServiceRequest.identifier = parseTaskIdentifier(taskIdentifier);
    }

    return cleanedServiceRequest;
}

export const constructBundleTask = (serviceRequest: ServiceRequest, primaryCondition: Condition, patientReference: Reference, serviceRequestReference: string, taskIdentifier?: string, practitionerRole?: PractitionerRole): Task => {
    const conditionCode = primaryCondition.code?.coding?.[0]
    if (!conditionCode) throw new Error("Primary condition has no coding, cannot create Task");

    const task = {
        resourceType: "Task",
        meta: {
            profile: [
                "http://santeonnl.github.io/shared-care-planning/StructureDefinition/SCPTask"
            ]
        },
        for: patientReference,
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
            reference: serviceRequestReference,
        },
    } as Task

    if (taskIdentifier) {
        task.identifier = parseTaskIdentifier(taskIdentifier);
    }

    if (practitionerRole) {

        task.contained = [practitionerRole]

        //TODO: This should be set, but currently breaks in orca. Leaving it as-is as this code will be removed with INT-558
        // task.owner = {
        //     type: "PractitionerRole",
        //     reference: `#${practitionerRole.id}`
        // }
    }

    return task
}

const parseTaskIdentifier = (taskIdentifier: string) => {
    const systemAndIdentifier = taskIdentifier.split("|");
    if (systemAndIdentifier.length !== 2) throw new Error("Invalid task identifier - expecting `system|identifier`");
    return [{
        system: systemAndIdentifier[0],
        value: systemAndIdentifier[1],
    }];
};

export const constructTaskBundle = (serviceRequest: ServiceRequest, primaryCondition: Condition, patient: Patient, practitionerRole?: PractitionerRole, taskIdentifier?: string): Bundle & {
    type: "transaction"
} => {
    const cleanedPatient = cleanPatient(patient);
    const patientReference: Reference = {
        type: "Patient",
        reference: "urn:uuid:patient",
        identifier: getPatientIdentifier(patient)!
    }
    const serviceRequestEntry = {
        fullUrl: "urn:uuid:serviceRequest",
        resource: cleanServiceRequest(serviceRequest, patient, patientReference.reference!, taskIdentifier),
        request: {
            method: "POST",
            url: "ServiceRequest",
            ifNoneExist: "",
        }
    }
    const taskEntry = {
        fullUrl: "urn:uuid:task",
        resource: constructBundleTask(serviceRequest, primaryCondition, patientReference, "urn:uuid:serviceRequest", taskIdentifier, practitionerRole),
        request: {
            method: "POST",
            url: "Task",
            ifNoneExist: "",
        }
    }
    if (taskIdentifier) {
        serviceRequestEntry.request.ifNoneExist = `identifier=${taskIdentifier}`
        taskEntry.request.ifNoneExist = `identifier=${taskIdentifier}`
    }

    const patientToken = encodeURIComponent(identifierToToken(getPatientIdentifier(patient)) || '');

    const bundle = {
        resourceType: "Bundle",
        type: "transaction",
        entry: [
            {
                fullUrl: "urn:uuid:patient",
                resource: cleanedPatient,
                request: {
                    method: "POST",
                    url: "Patient",
                    ifNoneExist: `identifier=${patientToken}`
                }
            },
            serviceRequestEntry,
            taskEntry
        ]
    }

    return bundle as Bundle & { type: "transaction" }
}

export const findQuestionnaireResponse = async (cpsClient: FhirClient, task?: Task, questionnaire?: Questionnaire) => {
    if (!task || !task.output || !questionnaire) return

    const questionnaireResponse = task.output.find((output) => {
        return output.valueReference?.reference?.startsWith(`QuestionnaireResponse/`)
    })

    if (!questionnaireResponse) return

    const questionnaireResponseId = questionnaireResponse.valueReference?.reference
    if (!questionnaireResponseId) return
    return await cpsClient.read({
        resourceType: "QuestionnaireResponse",
        id: questionnaireResponseId.split("/")[1]
    })
}

/**
 * Returns a string representation of an Identifier ONLY if both the system and value are set. Otherwise, returns undefined.
 * @param identifier
 * @returns
 */
export function identifierToToken(identifier?: Identifier) {

    if (!identifier?.system || !identifier.value) {
        return
    }

    return `${identifier.system}|${identifier.value}`
}

export type TaskProgress = {
    task: Task;
    questionnaireMap: Record<string, Questionnaire>;
    subTasks: Task[]
}

export const fetchTaskById = async (cpsClient: Client, taskId: string) => {
    return await cpsClient.read({resourceType: 'Task', id: taskId}) as Task;
}

export const fetchAllResources = async (taskId: string, cpsClient: Client): Promise<TaskProgress> => {
    const [task, subTasks] = await Promise.all([
        await fetchTaskById(cpsClient, taskId),
        await fetchSubTasks(cpsClient, taskId)
    ])

    const questionnaireMap = await fetchQuestionnaires(cpsClient, subTasks);
    return {task, questionnaireMap, subTasks};
}


export const fetchQuestionnaires = async (cpsClient: Client, subTasks: Task[]) => {
    const questionnaireMap: Record<string, Questionnaire> = {};
    await Promise.all(subTasks.map(async (task: Task) => {
        if (task.input && task.input.length > 0) {
            const input = task.input.find(input => input.valueReference?.reference?.startsWith("Questionnaire"));
            if (input && task.id && input.valueReference?.reference) {
                const questionnaireId = input.valueReference.reference;
                try {
                    questionnaireMap[task.id] = await cpsClient.read({
                        resourceType: "Questionnaire",
                        id: questionnaireId.split("/")[1]
                    }) as Questionnaire;
                } catch (error) {
                    throw new Error(`Failed to fetch questionnaire: ${error}`);
                }
            }
        }
    }));
    return questionnaireMap
}


export const fetchSubTasks = async (cpsClient: Client, taskId: string) => {
    try {
        const subTaskBundle = await cpsClient.search({
            resourceType: 'Task',
            searchParams: {"part-of": `Task/${taskId}`},
            headers: {"Cache-Control": "no-cache"},
            // @ts-ignore
            options: {postSearch: true}
        }) as Bundle<Task>;
        const subTasks = await fetchAllBundlePages(cpsClient, subTaskBundle);
        if (Array.isArray(subTasks) && subTasks.length > 0) {
            return subTasks;
        }
    } catch (error) {
        throw new Error(`Failed to fetch sub-tasks: ${error}`);
    }
    return [];
}


export function codingToMessage(codings: Coding[]): String[] {
    if (!codings.length) return [MessageType.Unknown];
    const messages: String[] = [];

    for (const coding of codings) {
        switch (coding.code) {
            case "E0001":
                messages.push(MessageType.NoEmail);
                break;
            case "E0002":
                messages.push(MessageType.NoPhone);
                break;
            case "E0003":
                messages.push(MessageType.InvalidEmail);
                break;
            case "E0004":
                messages.push(MessageType.InvalidPhone);
                break;
            default:
                messages.push(MessageType.Unknown + coding.code );
                break;
        }
    }
    return messages;
}

export enum MessageType {
    InvalidEmail = "Ongeldig e-mailadres. Controleer het e-mailadres van de patiënt in het EPD en probeer het opnieuw.",
    InvalidPhone = "Ongeldig telefoonnummer. Er is een Nederlands mobiel telefoonnummer nodig voor de aanmelding. Controleer het telefoonnummer in het EPD en probeer het opnieuw.",
    NoEmail = "Er is geen e-mailadres van de patiënt gevonden. Dit is nodig voor de aanmelding. Voeg het e-mailadres toe in het EPD en probeer het opnieuw.",
    NoPhone = "Er is geen telefoonnummer van de patiënt gevonden. Dit is nodig voor de aanmelding. Voeg het telefoonnummer toe in het EPD en probeer het opnieuw.",
    Unknown = "Er is een onbekende fout opgetreden. Probeer het later opnieuw of neem contact op met de systeembeheerder: functioneelbeheer@zorgbijjou.nl. Vermeld daarbij de volgende code: "
}