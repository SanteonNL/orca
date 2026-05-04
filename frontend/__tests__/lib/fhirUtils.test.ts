/**
 * @jest-environment node
 */

import {
    codingToMessage,
    MessageType,
    patientIdentifierSystem,
    getPatientIdentifier,
    findInBundle,
    identifierToToken,
    fetchAllBundlePages,
    createScpClient,
    createEhrClient,
    createCpsClient,
    constructBundleTask
} from '@/lib/fhirUtils';
import { Patient, Bundle, Task, Identifier, ServiceRequest, Condition, Reference, PractitionerRole } from 'fhir/r4';

describe('codingToMessage', () => {

    it('no email', () => {
        const codings = [{code: 'E0001'}]
        expect(codingToMessage(codings)).toStrictEqual([MessageType.NoEmail]);
    });
    it('no phone', () => {
        const codings = [{code: 'E0002'}]
        expect(codingToMessage(codings)).toStrictEqual([MessageType.NoPhone]);
    });
    it('invalid email', () => {
        const codings = [{code: 'E0003'}]
        expect(codingToMessage(codings)).toStrictEqual([MessageType.InvalidEmail]);
    })
    it('invalid phone', () => {
        const codings = [{code: 'E0004'}]
        expect(codingToMessage(codings)).toStrictEqual([MessageType.InvalidPhone]);
    });
    it('unknown code', () => {
        const codings = [{code: '1'}]
        expect(codingToMessage(codings)).toStrictEqual(['Er is een onbekende fout opgetreden. Probeer het later opnieuw of neem contact op met de systeembeheerder: functioneelbeheer@zorgbijjou.nl. Vermeld daarbij de volgende code: 1']);
    });

    it('empty array returns unknown', () => {
        const codings: any[] = []
        expect(codingToMessage(codings)).toStrictEqual([MessageType.Unknown]);
    });

    it('multiple codes', () => {
        const codings = [{code: 'E0001'}, {code: 'E0002'}]
        expect(codingToMessage(codings)).toStrictEqual([MessageType.NoEmail, MessageType.NoPhone]);
    });

});

describe('patientIdentifierSystem', () => {
    it('returns default BSN system when env var not set', () => {
        delete process.env.ORCA_PATIENT_IDENTIFIER_SYSTEM;
        expect(patientIdentifierSystem()).toBe('http://fhir.nl/fhir/NamingSystem/bsn');
    });

    it('returns configured system when env var is set', () => {
        process.env.ORCA_PATIENT_IDENTIFIER_SYSTEM = 'http://custom-system';
        expect(patientIdentifierSystem()).toBe('http://custom-system');
        delete process.env.ORCA_PATIENT_IDENTIFIER_SYSTEM;
    });
});

describe('getPatientIdentifier', () => {
    it('returns identifier matching the system', () => {
        const patient: Patient = {
            resourceType: 'Patient',
            identifier: [
                { system: 'http://fhir.nl/fhir/NamingSystem/bsn', value: '123456789' },
                { system: 'http://other-system', value: '987654321' }
            ]
        };
        
        const identifier = getPatientIdentifier(patient);
        expect(identifier?.system).toBe('http://fhir.nl/fhir/NamingSystem/bsn');
        expect(identifier?.value).toBe('123456789');
    });

    it('returns undefined when patient is undefined', () => {
        expect(getPatientIdentifier()).toBeUndefined();
    });

    it('returns undefined when no matching identifier', () => {
        const patient: Patient = {
            resourceType: 'Patient',
            identifier: [
                { system: 'http://other-system', value: '987654321' }
            ]
        };
        
        expect(getPatientIdentifier(patient)).toBeUndefined();
    });
});

describe('findInBundle', () => {
    it('finds resource by type in bundle', () => {
        const bundle: Bundle = {
            resourceType: 'Bundle',
            type: 'searchset',
            entry: [
                { resource: { resourceType: 'Patient', id: '123' } as Patient },
                { resource: { resourceType: 'Task', id: '456' } as Task }
            ]
        };

        const task = findInBundle('Task', bundle);
        expect(task?.resourceType).toBe('Task');
        expect((task as Task).id).toBe('456');
    });

    it('returns undefined when resource type not found', () => {
        const bundle: Bundle = {
            resourceType: 'Bundle',
            type: 'searchset',
            entry: [
                { resource: { resourceType: 'Patient', id: '123' } as Patient }
            ]
        };

        expect(findInBundle('Task', bundle)).toBeUndefined();
    });

    it('returns undefined when bundle is undefined', () => {
        expect(findInBundle('Task')).toBeUndefined();
    });
});

describe('identifierToToken', () => {
    it('returns formatted token with system and value', () => {
        const identifier: Identifier = {
            system: 'http://fhir.nl/fhir/NamingSystem/bsn',
            value: '123456789'
        };

        expect(identifierToToken(identifier)).toBe('http://fhir.nl/fhir/NamingSystem/bsn|123456789');
    });

    it('returns undefined when identifier is undefined', () => {
        expect(identifierToToken()).toBeUndefined();
    });

    it('returns undefined when system is missing', () => {
        const identifier: Identifier = {
            value: '123456789'
        };

        expect(identifierToToken(identifier)).toBeUndefined();
    });

    it('returns undefined when value is missing', () => {
        const identifier: Identifier = {
            system: 'http://fhir.nl/fhir/NamingSystem/bsn'
        };

        expect(identifierToToken(identifier)).toBeUndefined();
    });
});

describe('FHIR client creation', () => {
    beforeEach(() => {
        // Mock window.location for browser environment
        globalThis.window = { location: { origin: 'http://localhost:3000' } } as any;
    });

    afterEach(() => {
        delete (globalThis as any).window;
    });

    it('creates SCP client with correct baseUrl', () => {
        const client = createScpClient('tenant123');
        expect(client).toBeDefined();
        expect((client as any).baseUrl).toBe('http://localhost:3000/orca/cpc/tenant123/external/fhir');
    });

    it('creates EHR client with correct baseUrl', () => {
        const client = createEhrClient('tenant123');
        expect(client).toBeDefined();
        expect((client as any).baseUrl).toBe('http://localhost:3000/orca/cpc/tenant123/ehr/fhir');
    });

    it('creates CPS client with correct baseUrl and headers', () => {
        const client = createCpsClient('tenant123');
        expect(client).toBeDefined();
        expect((client as any).baseUrl).toBe('http://localhost:3000/orca/cpc/tenant123/external/fhir');
        expect((client as any).customHeaders).toEqual({ 'X-Scp-Fhir-Url': 'local-cps' });
    });
});

describe('fetchAllBundlePages', () => {
    it('fetches single page bundle', async () => {
        const mockClient = {
            nextPage: jest.fn()
        } as any;

        const mockBundle: Bundle<Patient> = {
            resourceType: 'Bundle',
            type: 'searchset',
            entry: [
                { resource: { resourceType: 'Patient', id: '1' } as Patient },
                { resource: { resourceType: 'Patient', id: '2' } as Patient }
            ]
        };

        const resources = await fetchAllBundlePages(mockClient, mockBundle);
        
        expect(resources).toHaveLength(2);
        expect(resources[0].id).toBe('1');
        expect(resources[1].id).toBe('2');
        expect(mockClient.nextPage).not.toHaveBeenCalled();
    });

    it('fetches multiple pages', async () => {
        const mockClient = {
            nextPage: jest.fn()
                .mockResolvedValueOnce({
                    resourceType: 'Bundle',
                    type: 'searchset',
                    entry: [
                        { resource: { resourceType: 'Patient', id: '3' } }
                    ],
                    link: []
                })
        } as any;

        const mockBundle: Bundle<Patient> = {
            resourceType: 'Bundle',
            type: 'searchset',
            entry: [
                { resource: { resourceType: 'Patient', id: '1' } as Patient }
            ],
            link: [
                { relation: 'next', url: 'http://example.com/next' }
            ]
        };

        const resources = await fetchAllBundlePages(mockClient, mockBundle);
        
        expect(resources).toHaveLength(2);
        expect(resources[0].id).toBe('1');
        expect(resources[1].id).toBe('3');
        expect(mockClient.nextPage).toHaveBeenCalledTimes(1);
    });
});

describe('constructBundleTask', () => {
    const mockServiceRequest: ServiceRequest = {
        resourceType: 'ServiceRequest',
        status: 'active',
        intent: 'order',
        code: {
            coding: [{
                system: 'http://example.com',
                code: 'test-code',
                display: 'Test Service'
            }]
        },
        requester: {
            identifier: {
                system: 'http://requester-system',
                value: 'req-123'
            }
        },
        performer: [{
            identifier: {
                system: 'http://performer-system',
                value: 'perf-123'
            }
        }],
        subject: { reference: 'Patient/123' }
    };

    const mockCondition: Condition = {
        resourceType: 'Condition',
        subject: { reference: 'Patient/123' },
        code: {
            coding: [{
                system: 'http://example.com',
                code: 'condition-code',
                display: 'Test Condition'
            }]
        }
    };

    const mockPatientReference: Reference = {
        type: 'Patient',
        reference: 'urn:uuid:patient'
    };

    it('constructs task with required fields', () => {
        const task = constructBundleTask(
            mockServiceRequest,
            mockCondition,
            mockPatientReference,
            'urn:uuid:serviceRequest'
        );

        expect(task.resourceType).toBe('Task');
        expect(task.status).toBe('requested');
        expect(task.intent).toBe('order');
        expect(task.for).toBe(mockPatientReference);
        expect(task.focus?.reference).toBe('urn:uuid:serviceRequest');
    });

    it('throws error when condition has no coding', () => {
        const invalidCondition: Condition = {
            resourceType: 'Condition',
            subject: { reference: 'Patient/123' }
        };

        expect(() => {
            constructBundleTask(
                mockServiceRequest,
                invalidCondition,
                mockPatientReference,
                'urn:uuid:serviceRequest'
            );
        }).toThrow('Primary condition has no coding, cannot create Task');
    });

    it('includes practitionerRole as contained resource', () => {
        const practitionerRole: PractitionerRole = {
            resourceType: 'PractitionerRole',
            id: 'pr-123'
        };

        const task = constructBundleTask(
            mockServiceRequest,
            mockCondition,
            mockPatientReference,
            'urn:uuid:serviceRequest',
            undefined,
            practitionerRole
        );

        expect(task.contained).toHaveLength(1);
        expect(task.contained?.[0]).toBe(practitionerRole);
    });

    it('includes task identifier when provided', () => {
        const task = constructBundleTask(
            mockServiceRequest,
            mockCondition,
            mockPatientReference,
            'urn:uuid:serviceRequest',
            'http://task-system|task-123'
        );

        expect(task.identifier).toHaveLength(1);
        expect(task.identifier?.[0].system).toBe('http://task-system');
        expect(task.identifier?.[0].value).toBe('task-123');
    });
});
