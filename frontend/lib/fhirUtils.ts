import Client from 'fhir-kit-client';
import {
    Bundle,
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

export const createCpsClient = () => {
    const baseUrl = process.env.NODE_ENV === "production"
        ? `${typeof window !== 'undefined' ? window.location.origin : ''}/orca/cpc/cps/fhir`
        : "http://localhost:9090/fhir";

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
        const result = await client.nextPage({
            bundle: {
                resourceType: 'Bundle',
                type: 'searchset',
                link: [{ relation: 'next', url: nextPageUrl }]
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
    const cleanedPatient = { ...patient, id: undefined }
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
    const cleanedServiceRequest = { ...serviceRequest, id: undefined };

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

    const patientIdentifier = getPatientIdentifier(patient)!;
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
                    ifNoneExist: `identifier=${patientIdentifier.system}|${patientIdentifier.value}`
                }
            },
            serviceRequestEntry,
            taskEntry
        ]
    }

    return bundle as Bundle & { type: "transaction" }
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

/**
 * Returns a string representation of an Identifier ONLY if both the system and value are set. Otherwise, returns undefined.
 * @param identifier 
 * @returns 
 */
export function identifierToString(identifier?: Identifier) {

    if (!identifier?.system || !identifier.value) {
        return
    }

    return `${identifier.system}|${identifier.value}`
}