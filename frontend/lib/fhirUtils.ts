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
    if (cleanedPatient.generalPractitioner?.[0]?.reference) {
        delete cleanedPatient.generalPractitioner[0].reference;
    }

    return cleanedPatient;
}

const cleanServiceRequest = (serviceRequest: ServiceRequest, patient: Patient, patientReference: string) => {
    // Clean up the ServiceRequest by removing relative references - the CPS won't understand them
    const cleanedServiceRequest = { ...serviceRequest, id: undefined };

    if (serviceRequest.subject?.identifier?.system === BSN_SYSTEM && serviceRequest.subject?.identifier?.value !== getBsn(patient)) {
        throw new Error("Subject BSN in service request differs from Patient BSN");
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

    return cleanedServiceRequest;
}

export const constructBundleTask = (serviceRequest: ServiceRequest, primaryCondition: Condition, patientReference: string, serviceRequestReference: string): Task => {
    const conditionCode = primaryCondition.code?.coding?.[0]
    if (!conditionCode) throw new Error("Primary condition has no coding, cannot create Task");

    return {
        resourceType: "Task",
        meta: {
            profile: [
                "http://santeonnl.github.io/shared-care-planning/StructureDefinition/SCPTask"
            ]
        },
        for: {
            reference: patientReference,
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
            reference: serviceRequestReference,
        },
    }
}

export const constructTaskBundle = (serviceRequest: ServiceRequest, primaryCondition: Condition, patient: Patient): Bundle & { type: "transaction" } => {
    const cleanedPatient = cleanPatient(patient);
    const cleanedServiceRequest = cleanServiceRequest(serviceRequest, patient, "urn:uuid:patient");
    const constructedTask = constructBundleTask(serviceRequest, primaryCondition, "urn:uuid:patient", "urn:uuid:serviceRequest");

    return {
        resourceType: "Bundle",
        type: "transaction",
        entry: [
            {
                fullUrl: "urn:uuid:patient",
                resource: cleanedPatient,
                request: {
                    method: "POST",
                    url: "Patient",
                    ifNoneExist: `identifier=http://fhir.nl/fhir/NamingSystem/bsn|${getBsn(patient)}`
                }
            },
            {
                fullUrl: "urn:uuid:serviceRequest",
                resource: cleanedServiceRequest,
                request: {
                    method: "POST",
                    url: "ServiceRequest"
                }
            },
            {
                fullUrl: "urn:uuid:task",
                resource: constructedTask,
                request: {
                    method: "POST",
                    url: "Task"
                }
            }
        ]
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

export default function organizationName(reference?: Reference) {

    if (!reference) {
        return "No Organization Reference found"
    }

    const displayName = reference.display;

    // If the identifier has no system or value, simply return the displayName, or "unknown" if no displayName is present.
    if (!reference.identifier || !reference.identifier.system || !reference.identifier.value) {
        return displayName || "unknown"
    }

    const isUraIdentifier = reference.identifier.system === 'http://fhir.nl/fhir/NamingSystem/ura'
    const identifierValue = isUraIdentifier ?
        `URA ${reference.identifier.value}` : `${reference.identifier.system}: ${reference.identifier.value}`;

    return displayName ? `${displayName} (${identifierValue})` : identifierValue
}