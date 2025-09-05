/**
 * @jest-environment node
 */

import type {
  Extension,
  Patient,
  Practitioner,
  Questionnaire
} from 'fhir/r4';
import type { InputParameters, OutputParameters } from '@aehrc/sdc-populate';
import { isInputParameters, isOutputParameters } from '@aehrc/sdc-populate';
import type { RequestConfig } from '@/app/utils/populateCallback';
import { createPopulateInputParameters } from '@/app/utils/populateInputParams';

// Mock dependencies
jest.mock('@aehrc/sdc-populate', () => ({
  isInputParameters: jest.fn(),
  isOutputParameters: jest.fn(),
  populate: jest.fn()
}));

jest.mock('@/app/utils/populateCallback', () => ({
  fetchResourceCallback: jest.fn()
}));

jest.mock('@/app/utils/populateInputParams', () => ({
  createPopulateInputParameters: jest.fn()
}));

const mockIsInputParameters = isInputParameters as jest.MockedFunction<typeof isInputParameters>;
const mockIsOutputParameters = isOutputParameters as jest.MockedFunction<typeof isOutputParameters>;
const mockCreatePopulateInputParameters = createPopulateInputParameters as jest.MockedFunction<typeof createPopulateInputParameters>;

// Import the functions after mocking dependencies
import {
  populateQuestionnaire,
  requestPopulate,
  isLaunchContext,
  getLaunchContexts,
  isSourceQuery,
  getSourceQueries,
  isXFhirQueryVariable,
  getQuestionnaireLevelXFhirQueryVariables
} from '@/app/utils/populate';

describe('populate utility functions', () => {
  const mockQuestionnaire: Questionnaire = {
    resourceType: 'Questionnaire',
    status: 'active',
    id: 'test-questionnaire'
  };

  const mockPatient: Patient = {
    resourceType: 'Patient',
    id: 'test-patient'
  };

  const mockUser: Practitioner = {
    resourceType: 'Practitioner',
    id: 'test-practitioner'
  };

  const mockRequestConfig: RequestConfig = {
    clientEndpoint: 'https://api.example.com',
    authToken: 'test-token'
  };

  beforeEach(() => {
    jest.clearAllMocks();
    jest.spyOn(console, 'log').mockImplementation(() => {});
    jest.spyOn(console, 'error').mockImplementation(() => {});
  });

  afterAll(() => {
    (console.log as jest.Mock).mockRestore();
    (console.error as jest.Mock).mockRestore();
    console.log = jest.fn();
    console.warn = jest.fn();
    console.error = jest.fn();
  });

  describe('isLaunchContext', () => {
    it('returns true for valid launch context with valueId', () => {
      const extension: Extension = {
        url: 'http://hl7.org/fhir/uv/sdc/StructureDefinition/sdc-questionnaire-launchContext',
        extension: [
          {
            url: 'name',
            valueId: 'patient'
          },
          {
            url: 'type',
            valueCode: 'Patient'
          }
        ]
      };

      expect(isLaunchContext(extension)).toBe(true);
    });

    it('returns true for valid launch context with valueCoding', () => {
      const extension: Extension = {
        url: 'http://hl7.org/fhir/uv/sdc/StructureDefinition/sdc-questionnaire-launchContext',
        extension: [
          {
            url: 'name',
            valueCoding: {
              code: 'patient'
            }
          },
          {
            url: 'type',
            valueCode: 'Patient'
          }
        ]
      };

      expect(isLaunchContext(extension)).toBe(true);
    });

    it('returns false for extension with wrong URL', () => {
      const extension: Extension = {
        url: 'http://example.com/wrong-url',
        extension: [
          {
            url: 'name',
            valueId: 'patient'
          },
          {
            url: 'type',
            valueCode: 'Patient'
          }
        ]
      };

      expect(isLaunchContext(extension)).toBe(false);
    });

    it('returns false for extension missing name', () => {
      const extension: Extension = {
        url: 'http://hl7.org/fhir/uv/sdc/StructureDefinition/sdc-questionnaire-launchContext',
        extension: [
          {
            url: 'type',
            valueCode: 'Patient'
          }
        ]
      };

      expect(isLaunchContext(extension)).toBe(false);
    });

    it('returns false for extension missing type', () => {
      const extension: Extension = {
        url: 'http://hl7.org/fhir/uv/sdc/StructureDefinition/sdc-questionnaire-launchContext',
        extension: [
          {
            url: 'name',
            valueId: 'patient'
          }
        ]
      };

      expect(isLaunchContext(extension)).toBe(false);
    });

    it('returns false for extension with no extensions', () => {
      const extension: Extension = {
        url: 'http://hl7.org/fhir/uv/sdc/StructureDefinition/sdc-questionnaire-launchContext'
      };

      expect(isLaunchContext(extension)).toBe(false);
    });
  });

  describe('getLaunchContexts', () => {
    it('returns launch contexts when valid extensions exist', () => {
      const questionnaireWithLaunchContext: Questionnaire = {
        ...mockQuestionnaire,
        extension: [
          {
            url: 'http://hl7.org/fhir/uv/sdc/StructureDefinition/sdc-questionnaire-launchContext',
            extension: [
              {
                url: 'name',
                valueId: 'patient'
              },
              {
                url: 'type',
                valueCode: 'Patient'
              }
            ]
          },
          {
            url: 'http://example.com/other-extension'
          }
        ]
      };

      const result = getLaunchContexts(questionnaireWithLaunchContext);

      expect(result).toHaveLength(1);
      expect(isLaunchContext(result[0])).toBe(true);
    });

    it('returns empty array when no extensions exist', () => {
      const result = getLaunchContexts(mockQuestionnaire);

      expect(result).toEqual([]);
    });

    it('returns empty array when no launch context extensions exist', () => {
      const questionnaireWithOtherExtensions: Questionnaire = {
        ...mockQuestionnaire,
        extension: [
          {
            url: 'http://example.com/other-extension'
          }
        ]
      };

      const result = getLaunchContexts(questionnaireWithOtherExtensions);

      expect(result).toEqual([]);
    });
  });

  describe('isSourceQuery', () => {
    it('returns true for valid source query extension', () => {
      const extension: Extension = {
        url: 'http://hl7.org/fhir/uv/sdc/StructureDefinition/sdc-questionnaire-sourceQueries',
        valueReference: {
          reference: 'Bundle/source-queries'
        }
      };

      expect(isSourceQuery(extension)).toBe(true);
    });

    it('returns false for extension with wrong URL', () => {
      const extension: Extension = {
        url: 'http://example.com/wrong-url',
        valueReference: {
          reference: 'Bundle/source-queries'
        }
      };

      expect(isSourceQuery(extension)).toBe(false);
    });

    it('returns false for extension without valueReference', () => {
      const extension: Extension = {
        url: 'http://hl7.org/fhir/uv/sdc/StructureDefinition/sdc-questionnaire-sourceQueries'
      };

      expect(isSourceQuery(extension)).toBe(false);
    });
  });

  describe('getSourceQueries', () => {
    it('returns source queries when valid extensions exist', () => {
      const questionnaireWithSourceQueries: Questionnaire = {
        ...mockQuestionnaire,
        extension: [
          {
            url: 'http://hl7.org/fhir/uv/sdc/StructureDefinition/sdc-questionnaire-sourceQueries',
            valueReference: {
              reference: 'Bundle/source-queries'
            }
          },
          {
            url: 'http://example.com/other-extension'
          }
        ]
      };

      const result = getSourceQueries(questionnaireWithSourceQueries);

      expect(result).toHaveLength(1);
      expect(isSourceQuery(result[0])).toBe(true);
    });

    it('returns empty array when no extensions exist', () => {
      const result = getSourceQueries(mockQuestionnaire);

      expect(result).toEqual([]);
    });

    it('returns empty array when no source query extensions exist', () => {
      const questionnaireWithOtherExtensions: Questionnaire = {
        ...mockQuestionnaire,
        extension: [
          {
            url: 'http://example.com/other-extension'
          }
        ]
      };

      const result = getSourceQueries(questionnaireWithOtherExtensions);

      expect(result).toEqual([]);
    });
  });

  describe('isXFhirQueryVariable', () => {
    it('returns true for valid x-fhir-query variable', () => {
      const extension: Extension = {
        url: 'http://hl7.org/fhir/StructureDefinition/variable',
        valueExpression: {
          name: 'testVariable',
          language: 'application/x-fhir-query',
          expression: 'Patient?active=true'
        }
      };

      expect(isXFhirQueryVariable(extension)).toBe(true);
    });

    it('returns false for extension with wrong URL', () => {
      const extension: Extension = {
        url: 'http://example.com/wrong-url',
        valueExpression: {
          name: 'testVariable',
          language: 'application/x-fhir-query',
          expression: 'Patient?active=true'
        }
      };

      expect(isXFhirQueryVariable(extension)).toBe(false);
    });

    it('returns false for extension without valueExpression', () => {
      const extension: Extension = {
        url: 'http://hl7.org/fhir/StructureDefinition/variable'
      };

      expect(isXFhirQueryVariable(extension)).toBe(false);
    });

    it('returns false for extension without name in valueExpression', () => {
      const extension: Extension = {
        url: 'http://hl7.org/fhir/StructureDefinition/variable',
        valueExpression: {
          language: 'application/x-fhir-query',
          expression: 'Patient?active=true'
        }
      };

      expect(isXFhirQueryVariable(extension)).toBe(false);
    });

    it('returns false for extension with wrong language', () => {
      const extension: Extension = {
        url: 'http://hl7.org/fhir/StructureDefinition/variable',
        valueExpression: {
          name: 'testVariable',
          language: 'text/fhirpath',
          expression: 'Patient.active'
        }
      };

      expect(isXFhirQueryVariable(extension)).toBe(false);
    });

    it('returns false for extension without expression', () => {
      const extension: Extension = {
        url: 'http://hl7.org/fhir/StructureDefinition/variable',
        valueExpression: {
          name: 'testVariable',
          language: 'application/x-fhir-query'
        }
      };

      expect(isXFhirQueryVariable(extension)).toBe(false);
    });
  });

  describe('getQuestionnaireLevelXFhirQueryVariables', () => {
    it('returns x-fhir-query variables when valid extensions exist', () => {
      const questionnaireWithVariables: Questionnaire = {
        ...mockQuestionnaire,
        extension: [
          {
            url: 'http://hl7.org/fhir/StructureDefinition/variable',
            valueExpression: {
              name: 'testVariable',
              language: 'application/x-fhir-query',
              expression: 'Patient?active=true'
            }
          },
          {
            url: 'http://example.com/other-extension'
          }
        ]
      };

      const result = getQuestionnaireLevelXFhirQueryVariables(questionnaireWithVariables);

      expect(result).toHaveLength(1);
      expect(isXFhirQueryVariable(result[0])).toBe(true);
    });

    it('returns empty array when no extensions exist', () => {
      const result = getQuestionnaireLevelXFhirQueryVariables(mockQuestionnaire);

      expect(result).toEqual([]);
    });

    it('returns empty array when no x-fhir-query variable extensions exist', () => {
      const questionnaireWithOtherExtensions: Questionnaire = {
        ...mockQuestionnaire,
        extension: [
          {
            url: 'http://example.com/other-extension'
          }
        ]
      };

      const result = getQuestionnaireLevelXFhirQueryVariables(questionnaireWithOtherExtensions);

      expect(result).toEqual([]);
    });
  });

  describe('populateQuestionnaire', () => {
    it('returns success false when no launch contexts, source queries, or variables found', async () => {
      const result = await populateQuestionnaire(
        mockQuestionnaire,
        mockPatient,
        mockUser,
        mockRequestConfig
      );

      expect(result.populateSuccess).toBe(false);
      expect(result.populateResult).toBe(null);
      expect(console.log).toHaveBeenCalledWith(
        'No launch contexts, source queries, or questionnaire-level variables found.'
      );
    });

    it('returns success false when input parameters cannot be created', async () => {
      const questionnaireWithExtensions: Questionnaire = {
        ...mockQuestionnaire,
        extension: [
          {
            url: 'http://hl7.org/fhir/uv/sdc/StructureDefinition/sdc-questionnaire-launchContext',
            extension: [
              {
                url: 'name',
                valueId: 'patient'
              },
              {
                url: 'type',
                valueCode: 'Patient'
              }
            ]
          }
        ]
      };

      mockCreatePopulateInputParameters.mockReturnValue(null);

      const result = await populateQuestionnaire(
        questionnaireWithExtensions,
        mockPatient,
        mockUser,
        mockRequestConfig
      );

      expect(result.populateSuccess).toBe(false);
      expect(result.populateResult).toBe(null);
      expect(console.log).toHaveBeenCalledWith('No input parameters created.');
    });

    it('returns success false when input parameters are invalid', async () => {
      const questionnaireWithExtensions: Questionnaire = {
        ...mockQuestionnaire,
        extension: [
          {
            url: 'http://hl7.org/fhir/uv/sdc/StructureDefinition/sdc-questionnaire-launchContext',
            extension: [
              {
                url: 'name',
                valueId: 'patient'
              },
              {
                url: 'type',
                valueCode: 'Patient'
              }
            ]
          }
        ]
      };

      const mockInputParameters = { resourceType: 'Parameters' } as InputParameters;
      mockCreatePopulateInputParameters.mockReturnValue(mockInputParameters);
      mockIsInputParameters.mockReturnValue(false);

      const result = await populateQuestionnaire(
        questionnaireWithExtensions,
        mockPatient,
        mockUser,
        mockRequestConfig
      );

      expect(result.populateSuccess).toBe(false);
      expect(result.populateResult).toBe(null);
      expect(console.log).toHaveBeenCalledWith('Input parameters are not valid.');
    });
  });

  describe('requestPopulate', () => {
    const mockInputParameters: InputParameters = {
      resourceType: 'Parameters',
      parameter: [
        {
          name: 'questionnaire',
          resource: mockQuestionnaire
        },
        {
          name: 'subject',
          valueReference: {
            reference: 'Patient/test-patient'
          }
        }
      ]
    };

    beforeEach(() => {
      jest.clearAllMocks();
    });


    it('returns valid output parameters when operation succeeds', async () => {
      const { populate } = require('@aehrc/sdc-populate');
      const mockOutputParameters: OutputParameters = {
        resourceType: 'Parameters',
        parameter: [
          {
            name: 'response',
            resource: {
              resourceType: 'QuestionnaireResponse',
              status: 'completed'
            }
          }
        ]
      };

      populate.mockResolvedValue(mockOutputParameters);
      mockIsOutputParameters.mockReturnValue(true);

      const result = await requestPopulate(mockInputParameters, mockRequestConfig);

      expect(result).toBe(mockOutputParameters);
      expect(console.log).toHaveBeenCalledWith('Requesting population with input parameters:', mockInputParameters);
    });

    it('returns invalid operation outcome when output parameters are invalid', async () => {
      const { populate } = require('@aehrc/sdc-populate');
      const invalidOutput = { invalid: 'data' };

      populate.mockResolvedValue(invalidOutput);
      mockIsOutputParameters.mockReturnValue(false);

      const result = await requestPopulate(mockInputParameters, mockRequestConfig);

      expect(result).toEqual({
        resourceType: 'OperationOutcome',
        issue: [
          {
            severity: 'error',
            code: 'invalid',
            details: {
              text: 'Output parameters do not match the specification.'
            }
          }
        ]
      });
    });

    it('returns error operation outcome when exception is thrown', async () => {
      const { populate } = require('@aehrc/sdc-populate');
      const error = new Error('Network error');

      populate.mockRejectedValue(error);

      const result = await requestPopulate(mockInputParameters, mockRequestConfig);

      expect(result).toEqual({
        resourceType: 'OperationOutcome',
        issue: [
          {
            severity: 'error',
            code: 'unknown',
            details: { text: 'An unknown error occurred.' }
          }
        ]
      });
      expect(console.error).toHaveBeenCalledWith('Error:', error);
    });
  });
});
