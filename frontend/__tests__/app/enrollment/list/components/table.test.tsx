import { render, screen } from '@testing-library/react';
import '@testing-library/jest-dom';
import TaskOverviewTable from '@/app/enrollment/list/components/table';
import useEnrollmentStore from '@/lib/store/enrollment-store';
import useCpsClient from '@/hooks/use-cps-client';
import * as fhirUtils from '@/lib/fhirUtils';

jest.mock('@/lib/store/enrollment-store');
jest.mock('@/hooks/use-cps-client');
jest.mock('@/lib/fhirUtils');

const mockPatient = {
  id: 'patient-1',
  identifier: [
    { system: 'http://fhir.nl/fhir/NamingSystem/bsn', value: '123456789' }
  ]
};

const mockTasks = [
  {
    id: 'task-1',
    owner: { display: 'Dr. Smith' },
    focus: { display: 'Cardiologie consult' },
    status: 'requested',
    lastModified: '2024-01-15'
  },
  {
    id: 'task-2',
    owner: { display: 'Dr. Johnson' },
    focus: { display: 'Bloedonderzoek' },
    status: 'completed',
    lastModified: '2024-01-14'
  }
];

beforeEach(() => {
  jest.clearAllMocks();
  (useEnrollmentStore as jest.Mock).mockReturnValue({ patient: mockPatient });
  (useCpsClient as jest.Mock).mockReturnValue({ search: jest.fn() });
  (fhirUtils.getPatientIdentifier as jest.Mock).mockReturnValue(mockPatient.identifier[0]);
});

describe('TaskOverviewTable', () => {
it('renders table headers correctly', () => {
  render(<TaskOverviewTable />);
  expect(screen.getByText('Uitvoerder')).toBeInTheDocument();
  expect(screen.getByText('Verzoek')).toBeInTheDocument();
  expect(screen.getByText('Status')).toBeInTheDocument();
  expect(screen.getByText('Datum')).toBeInTheDocument();
});

it('calls cpsClient search with correct patient identifier when patient has BSN', () => {
  const mockSearch = jest.fn();
  (useCpsClient as jest.Mock).mockReturnValue({ search: mockSearch });

  render(<TaskOverviewTable />);

  expect(mockSearch).toHaveBeenCalledWith({
    resourceType: 'Task',
    searchParams: {
      'patient': 'http://fhir.nl/fhir/NamingSystem/bsn|123456789'
    }
  });
});

it('falls back to first available identifier when patient has no BSN', () => {
  const patientWithoutBSN = {
    id: 'patient-2',
    identifier: [
      { system: 'http://example.com/mrn', value: 'MRN123' }
    ]
  };
  const mockSearch = jest.fn();
  (useEnrollmentStore as jest.Mock).mockReturnValue({ patient: patientWithoutBSN });
  (useCpsClient as jest.Mock).mockReturnValue({ search: mockSearch });
  (fhirUtils.getPatientIdentifier as jest.Mock).mockReturnValue(null);

  render(<TaskOverviewTable />);

  expect(mockSearch).toHaveBeenCalledWith({
    resourceType: 'Task',
    searchParams: {
      'patient': 'http://example.com/mrn|MRN123'
    }
  });
});

it('does not call search when patient is not available', () => {
  const mockSearch = jest.fn();
  (useEnrollmentStore as jest.Mock).mockReturnValue({ patient: null });
  (useCpsClient as jest.Mock).mockReturnValue({ search: mockSearch });

  render(<TaskOverviewTable />);

  expect(mockSearch).not.toHaveBeenCalled();
});

it('throws error when patient has no identifiers', () => {
  const patientWithoutIdentifiers = { id: 'patient-3', identifier: [] };
  (useEnrollmentStore as jest.Mock).mockReturnValue({ patient: patientWithoutIdentifiers });
  (fhirUtils.getPatientIdentifier as jest.Mock).mockReturnValue(null);

  expect(() => render(<TaskOverviewTable />)).toThrow('No patient identifier found for the patient');
})});