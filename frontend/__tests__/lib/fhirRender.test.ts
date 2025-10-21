/**
 * @jest-environment node
 */

import { Patient } from 'fhir/r4';
import { patientName } from '@/lib/fhirRender';

describe('patientName', () => {

  describe('when patient has no name array', () => {
    it('should return "(no name)" when patient.name is undefined', () => {
      const patient: Patient = {
        resourceType: 'Patient'
      };

      expect(patientName(patient)).toBe('(no name)');
    });

    it('should return "(no name)" when patient.name is null', () => {
      const patient: Patient = {
        resourceType: 'Patient',
        name: null as any
      };

      expect(patientName(patient)).toBe('(no name)');
    });

    it('should return "(no name)" when patient.name is empty array', () => {
      const patient: Patient = {
        resourceType: 'Patient',
        name: []
      };

      expect(patientName(patient)).toBe('(no name)');
    });
  });

  describe('when patient has name array with valid given and family names', () => {
    it('should return "FirstName FamilyName" when both given[0] and family are present', () => {
      const patient: Patient = {
        resourceType: 'Patient',
        name: [{
          given: ['John'],
          family: 'Doe'
        }]
      };

      expect(patientName(patient)).toBe('John Doe');
    });

    it('should return "FirstName FamilyName" when multiple given names are present (uses first one)', () => {
      const patient: Patient = {
        resourceType: 'Patient',
        name: [{
          given: ['John', 'Michael', 'J.'],
          family: 'Doe'
        }]
      };

      expect(patientName(patient)).toBe('John Doe');
    });

    it('should return "FirstName FamilyName" when multiple given names are present (uses first one) and text field is present', () => {
      const patient: Patient = {
        resourceType: 'Patient',
        name: [{
          given: ['John', 'Michael', 'J.'],
          family: 'Doe',
          text: 'John Michael J. Doe'
        }]
      };

      expect(patientName(patient)).toBe('John Doe');
    });

    it('should handle names with special characters', () => {
      const patient: Patient = {
        resourceType: 'Patient',
        name: [{
          given: ['José'],
          family: 'García-López'
        }]
      };

      expect(patientName(patient)).toBe('José García-López');
    });

    it('should handle names with spaces', () => {
      const patient: Patient = {
        resourceType: 'Patient',
        name: [{
          given: ['Mary Jane'],
          family: 'van der Berg'
        }]
      };

      expect(patientName(patient)).toBe('Mary Jane van der Berg');
    });
  });

  describe('when patient has incomplete given/family names', () => {
    it('should fallback to text when given is undefined but family is present', () => {
      const patient: Patient = {
        resourceType: 'Patient',
        name: [{
          family: 'Doe',
          text: 'Mr. John Doe'
        }]
      };

      expect(patientName(patient)).toBe('Mr. John Doe');
    });

    it('should fallback to text when given is empty array but family is present', () => {
      const patient: Patient = {
        resourceType: 'Patient',
        name: [{
          given: [],
          family: 'Doe',
          text: 'Mr. Doe'
        }]
      };

      expect(patientName(patient)).toBe('Mr. Doe');
    });

    it('should fallback to text when given is present but family is undefined', () => {
      const patient: Patient = {
        resourceType: 'Patient',
        name: [{
          given: ['John'],
          text: 'John (no last name)'
        }]
      };

      expect(patientName(patient)).toBe('John (no last name)');
    });

    it('should fallback to text when given is present but family is null', () => {
      const patient: Patient = {
        resourceType: 'Patient',
        name: [{
          given: ['John'],
          family: null as any,
          text: 'John (no family name)'
        }]
      };

      expect(patientName(patient)).toBe('John (no family name)');
    });

    it('should fallback to text when given is present but family is empty string', () => {
      const patient: Patient = {
        resourceType: 'Patient',
        name: [{
          given: ['John'],
          family: '',
          text: 'John (empty family name)'
        }]
      };

      expect(patientName(patient)).toBe('John (empty family name)');
    });

    it('should fallback to text when both given and family are undefined', () => {
      const patient: Patient = {
        resourceType: 'Patient',
        name: [{
          text: 'Unknown Patient Name'
        }]
      };

      expect(patientName(patient)).toBe('Unknown Patient Name');
    });
  });

  describe('when patient has only text field', () => {
    it('should return text when it contains a full name', () => {
      const patient: Patient = {
        resourceType: 'Patient',
        name: [{
          text: 'Dr. Jane Smith, MD'
        }]
      };

      expect(patientName(patient)).toBe('Dr. Jane Smith, MD');
    });

    it('should return text when it contains unusual formatting', () => {
      const patient: Patient = {
        resourceType: 'Patient',
        name: [{
          text: 'Smith, John Q.'
        }]
      };

      expect(patientName(patient)).toBe('Smith, John Q.');
    });
  });

  describe('when patient name has no usable information', () => {
    it('should return "(empty name)" when name object has no given, family, or text', () => {
      const patient: Patient = {
        resourceType: 'Patient',
        name: [{}]
      };

      expect(patientName(patient)).toBe('(empty name)');
    });

    it('should return "(empty name)" when text is undefined and given/family are incomplete', () => {
      const patient: Patient = {
        resourceType: 'Patient',
        name: [{
          given: [],
          family: undefined
        }]
      };

      expect(patientName(patient)).toBe('(empty name)');
    });

    it('should return "(empty name)" when text is null and given/family are incomplete', () => {
      const patient: Patient = {
        resourceType: 'Patient',
        name: [{
          given: ['John'],
          family: '',
          text: null as any
        }]
      };

      expect(patientName(patient)).toBe('(empty name)');
    });

    it('should return "(empty name)" when text is empty string and given/family are incomplete', () => {
      const patient: Patient = {
        resourceType: 'Patient',
        name: [{
          given: undefined,
          family: 'Doe',
          text: ''
        }]
      };

      expect(patientName(patient)).toBe('(empty name)');
    });
  });

  describe('edge cases and complex scenarios', () => {
    it('should use first name object when multiple name objects are present', () => {
      const patient: Patient = {
        resourceType: 'Patient',
        name: [
          {
            given: ['John'],
            family: 'Doe'
          },
          {
            given: ['Johnny'],
            family: 'Doe',
            use: 'nickname'
          }
        ]
      };

      expect(patientName(patient)).toBe('John Doe');
    });

    it('should handle first name object with no data, but second has data', () => {
      const patient: Patient = {
        resourceType: 'Patient',
        name: [
          {},
          {
            given: ['Jane'],
            family: 'Smith'
          }
        ]
      };

      expect(patientName(patient)).toBe('(empty name)');
    });

    it('should handle whitespace-only given name', () => {
      const patient: Patient = {
        resourceType: 'Patient',
        name: [{
          given: ['   '],
          family: 'Doe'
        }]
      };

      expect(patientName(patient)).toBe('    Doe');
    });

    it('should handle whitespace-only family name', () => {
      const patient: Patient = {
        resourceType: 'Patient',
        name: [{
          given: ['John'],
          family: '   ',
          text: 'John (whitespace family)'
        }]
      };

      expect(patientName(patient)).toBe('John    ');
    });

    it('should handle whitespace-only text field', () => {
      const patient: Patient = {
        resourceType: 'Patient',
        name: [{
          text: '   '
        }]
      };

      expect(patientName(patient)).toBe('   ');
    });

    it('should handle numeric values in names', () => {
      const patient: Patient = {
        resourceType: 'Patient',
        name: [{
          given: ['John2'],
          family: 'Doe3'
        }]
      };

      expect(patientName(patient)).toBe('John2 Doe3');
    });
  });
});
