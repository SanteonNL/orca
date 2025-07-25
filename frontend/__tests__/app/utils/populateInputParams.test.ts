/**
 * @jest-environment node
 */

import { createPopulateInputParameters } from '@/app/utils/populateInputParams';
import type { Patient, Practitioner, Questionnaire } from 'fhir/r4';
import type { LaunchContext, QuestionnaireLevelXFhirQueryVariable, SourceQuery } from '@/app/utils/populate';

describe('createPopulateInputParameters', () => {
  const mockPatient: Patient = {
    resourceType: 'Patient',
    id: 'patient-123'
  };

  const mockPractitioner: Practitioner = {
    resourceType: 'Practitioner',
    id: 'practitioner-456'
  };

  const mockQuestionnaire: Questionnaire = {
    resourceType: 'Questionnaire',
    id: 'questionnaire-789',
    status: 'active'
  };

  const mockFhirPathContext: Record<string, any> = {};

  beforeEach(() => {
    Object.keys(mockFhirPathContext).forEach(key => delete mockFhirPathContext[key]);
  });

  it('creates basic parameters with questionnaire and subject when minimal inputs provided', () => {
    const result = createPopulateInputParameters(
      mockQuestionnaire,
      mockPatient,
      mockPractitioner,
      [],
      [],
      [],
      mockFhirPathContext
    );

    expect(result).toEqual({
      resourceType: 'Parameters',
      parameter: [
        {
          name: 'questionnaire',
          resource: mockQuestionnaire
        },
        {
          name: 'subject',
          valueReference: {
            type: 'Patient',
            reference: 'Patient/patient-123'
          }
        },
        {
          name: 'local',
          valueBoolean: false
        }
      ]
    });
  });

  it('returns null when patient subject cannot be created', () => {
    const questionnaireWithRestrictedSubjects: Questionnaire = {
      ...mockQuestionnaire,
      subjectType: ['Organization', 'Practitioner']
    };

    const result = createPopulateInputParameters(
      questionnaireWithRestrictedSubjects,
      mockPatient,
      mockPractitioner,
      [],
      [],
      [],
      mockFhirPathContext
    );

    expect(result).toBeNull();
  });

  it('includes canonical parameter when questionnaire has url', () => {
    const questionnaireWithUrl: Questionnaire = {
      ...mockQuestionnaire,
      url: 'https://example.com/questionnaire'
    };

    const result = createPopulateInputParameters(
      questionnaireWithUrl,
      mockPatient,
      mockPractitioner,
      [],
      [],
      [],
      mockFhirPathContext
    );

    expect(result?.parameter).toContainEqual({
      name: 'canonical',
      valueString: 'https://example.com/questionnaire'
    });
  });

  it('adds launch context parameters for patient context', () => {
    const mockLaunchContext: LaunchContext = {
      extension: [
        { valueId: 'patient' },
        { valueCode: 'Patient' }
      ]
    } as LaunchContext;

    const result = createPopulateInputParameters(
      mockQuestionnaire,
      mockPatient,
      mockPractitioner,
      [mockLaunchContext],
      [],
      [],
      mockFhirPathContext
    );

    expect(result?.parameter).toContainEqual({
      name: 'context',
      part: [
        {
          name: 'name',
          valueString: 'patient'
        },
        {
          name: 'content',
          resource: mockPatient
        }
      ]
    });
    expect(mockFhirPathContext.patient).toBe(mockPatient);
  });

  it('adds launch context parameters for user context', () => {
    const mockLaunchContext: LaunchContext = {
      extension: [
        { valueId: 'user' },
        { valueCode: 'Practitioner' }
      ]
    } as LaunchContext;

    const result = createPopulateInputParameters(
      mockQuestionnaire,
      mockPatient,
      mockPractitioner,
      [mockLaunchContext],
      [],
      [],
      mockFhirPathContext
    );

    expect(result?.parameter).toContainEqual({
      name: 'context',
      part: [
        {
          name: 'name',
          valueString: 'user'
        },
        {
          name: 'content',
          resource: mockPractitioner
        }
      ]
    });
    expect(mockFhirPathContext.user).toBe(mockPractitioner);
  });

  it('uses valueCoding code when valueId is not available in launch context', () => {
    const mockLaunchContext: LaunchContext = {
      extension: [
        { valueCoding: { code: 'patient' } },
        { valueCode: 'Patient' }
      ]
    } as LaunchContext;

    const result = createPopulateInputParameters(
      mockQuestionnaire,
      mockPatient,
      mockPractitioner,
      [mockLaunchContext],
      [],
      [],
      mockFhirPathContext
    );

    expect(result?.parameter).toContainEqual({
      name: 'context',
      part: [
        {
          name: 'name',
          valueString: 'patient'
        },
        {
          name: 'content',
          resource: mockPatient
        }
      ]
    });
  });

  it('skips launch context when name cannot be determined', () => {
    const mockLaunchContext: LaunchContext = {
      extension: [
        {},
        { valueCode: 'Patient' }
      ]
    } as LaunchContext;

    const result = createPopulateInputParameters(
      mockQuestionnaire,
      mockPatient,
      mockPractitioner,
      [mockLaunchContext],
      [],
      [],
      mockFhirPathContext
    );

    const contextParams = result?.parameter?.filter(p => p.name === 'context');
    expect(contextParams).toHaveLength(0);
  });

  it('skips unsupported launch context resource types', () => {
    const mockLaunchContext: LaunchContext = {
      extension: [
        { valueId: 'organization' },
        { valueCode: 'Organization' }
      ]
    } as LaunchContext;

    const result = createPopulateInputParameters(
      mockQuestionnaire,
      mockPatient,
      mockPractitioner,
      [mockLaunchContext],
      [],
      [],
      mockFhirPathContext
    );

    const contextParams = result?.parameter?.filter(p => p.name === 'context');
    expect(contextParams).toHaveLength(0);
  });

  it('adds source query parameters for contained references', () => {
    const mockSourceQuery: SourceQuery = {
      valueReference: {
        reference: '#contained-resource'
      }
    } as SourceQuery;

    const result = createPopulateInputParameters(
      mockQuestionnaire,
      mockPatient,
      mockPractitioner,
      [],
      [mockSourceQuery],
      [],
      mockFhirPathContext
    );

    expect(result?.parameter).toContainEqual({
      name: 'context',
      part: [
        {
          name: 'name',
          valueString: 'contained-resource'
        },
        {
          name: 'content',
          valueReference: { reference: '#contained-resource' }
        }
      ]
    });
  });

  it('adds source query parameters for non-contained references with fallback naming', () => {
    const mockSourceQuery: SourceQuery = {
      valueReference: {
        reference: 'Patient/external-123'
      }
    } as SourceQuery;

    const result = createPopulateInputParameters(
      mockQuestionnaire,
      mockPatient,
      mockPractitioner,
      [],
      [mockSourceQuery],
      [],
      mockFhirPathContext
    );

    expect(result?.parameter).toContainEqual({
      name: 'context',
      part: [
        {
          name: 'name',
          valueString: 'sourceQuery0'
        },
        {
          name: 'content',
          valueReference: { reference: 'Patient/external-123' }
        }
      ]
    });
  });

  it('adds questionnaire-level variable parameters', () => {
    const mockVariable: QuestionnaireLevelXFhirQueryVariable = {
      valueExpression: {
        name: 'patientConditions',
        expression: 'Condition?patient={{%patient.id}}'
      }
    } as QuestionnaireLevelXFhirQueryVariable;

    const result = createPopulateInputParameters(
      mockQuestionnaire,
      mockPatient,
      mockPractitioner,
      [],
      [],
      [mockVariable],
      mockFhirPathContext
    );

    expect(result?.parameter).toContainEqual({
      name: 'context',
      part: [
        {
          name: 'name',
          valueString: 'patientConditions'
        },
        {
          name: 'content',
          valueReference: {
            reference: 'Condition?patient={{%patient.id}}',
            type: 'Condition'
          }
        }
      ]
    });
  });

  it('handles questionnaire-level variables with complex queries', () => {
    const mockVariable: QuestionnaireLevelXFhirQueryVariable = {
      valueExpression: {
        name: 'observations',
        expression: 'Observation?subject=Patient/123&category=vital-signs'
      }
    } as QuestionnaireLevelXFhirQueryVariable;

    const result = createPopulateInputParameters(
      mockQuestionnaire,
      mockPatient,
      mockPractitioner,
      [],
      [],
      [mockVariable],
      mockFhirPathContext
    );

    expect(result?.parameter).toContainEqual({
      name: 'context',
      part: [
        {
          name: 'name',
          valueString: 'observations'
        },
        {
          name: 'content',
          valueReference: {
            reference: 'Observation?subject=Patient/123&category=vital-signs',
            type: 'Observation'
          }
        }
      ]
    });
  });

  it('combines all context types when provided', () => {
    const mockLaunchContext: LaunchContext = {
      extension: [
        { valueId: 'patient' },
        { valueCode: 'Patient' }
      ]
    } as LaunchContext;

    const mockSourceQuery: SourceQuery = {
      valueReference: {
        reference: '#contained-resource'
      }
    } as SourceQuery;

    const mockVariable: QuestionnaireLevelXFhirQueryVariable = {
      valueExpression: {
        name: 'conditions',
        expression: 'Condition?patient={{%patient.id}}'
      }
    } as QuestionnaireLevelXFhirQueryVariable;

    const result = createPopulateInputParameters(
      mockQuestionnaire,
      mockPatient,
      mockPractitioner,
      [mockLaunchContext],
      [mockSourceQuery],
      [mockVariable],
      mockFhirPathContext
    );

    const contextParams = result?.parameter?.filter(p => p.name === 'context');
    expect(contextParams).toHaveLength(3);
  });

  it('always includes local parameter set to false', () => {
    const result = createPopulateInputParameters(
      mockQuestionnaire,
      mockPatient,
      mockPractitioner,
      [],
      [],
      [],
      mockFhirPathContext
    );

    expect(result?.parameter).toContainEqual({
      name: 'local',
      valueBoolean: false
    });
  });

  it('creates patient subject when questionnaire has no subject type restrictions', () => {
    const questionnaireWithoutSubjectType: Questionnaire = {
      ...mockQuestionnaire,
      subjectType: undefined
    };

    const result = createPopulateInputParameters(
      questionnaireWithoutSubjectType,
      mockPatient,
      mockPractitioner,
      [],
      [],
      [],
      mockFhirPathContext
    );

    expect(result?.parameter).toContainEqual({
      name: 'subject',
      valueReference: {
        type: 'Patient',
        reference: 'Patient/patient-123'
      }
    });
  });

  it('creates patient subject when questionnaire explicitly allows Patient subject type', () => {
    const questionnaireWithPatientSubject: Questionnaire = {
      ...mockQuestionnaire,
      subjectType: ['Patient', 'Organization']
    };

    const result = createPopulateInputParameters(
      questionnaireWithPatientSubject,
      mockPatient,
      mockPractitioner,
      [],
      [],
      [],
      mockFhirPathContext
    );

    expect(result?.parameter).toContainEqual({
      name: 'subject',
      valueReference: {
        type: 'Patient',
        reference: 'Patient/patient-123'
      }
    });
  });
});
