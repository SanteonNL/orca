import {act, render, screen} from '@testing-library/react';
import '@testing-library/jest-dom';
import TaskOverviewTable from '@/app/enrollment/list/components/table';
import useEnrollment from '@/app/hooks/enrollment-hook';
import * as fhirUtils from '@/lib/fhirUtils';
import {useRouter} from 'next/navigation';

jest.mock('@/app/hooks/enrollment-hook');
jest.mock('@/lib/fhirUtils');
jest.mock('next/navigation');

const mockPatient = {
    id: 'patient-1',
    identifier: [
        {system: 'http://fhir.nl/fhir/NamingSystem/bsn', value: '123456789'}
    ]
};

beforeEach(() => {
    jest.resetAllMocks();
    (useEnrollment as jest.Mock).mockReturnValue({patient: mockPatient});
    (fhirUtils.getPatientIdentifier as jest.Mock).mockReturnValue(mockPatient.identifier[0]);
    (fhirUtils.identifierToToken as jest.Mock).mockReturnValue('http://fhir.nl/fhir/NamingSystem/bsn|123456789');
    (useRouter as jest.Mock).mockReturnValue({
        push: jest.fn(),
    });

    mockSearch.mockResolvedValue({ entry: [] })
    mockUseClients.mockReturnValue({
      cpsClient: { search: mockSearch },
    })
});

const mockSearch = jest.fn()
const mockUseClients = jest.fn()

jest.mock('@/app/hooks/context-hook', () => ({
  useLaunchContext: () => ({
      launchContext: {
        taskIdentifier: 'task-id-123',
      },
    }),
  useClients: () => mockUseClients(),
}))

describe('TaskOverviewTable', () => {
    it('renders table headers correctly', async () => {
        await act(async () => {
            render(<TaskOverviewTable/>);
        });
        expect(screen.getByText('Uitvoerder')).toBeInTheDocument();
        expect(screen.getByText('Type')).toBeInTheDocument();
        expect(screen.getByText('Status')).toBeInTheDocument();
        expect(screen.getByText('Datum')).toBeInTheDocument();
    });

    it('calls cpsClient search with correct patient identifier when patient has BSN', async () => {
        // Mock the patient search response first, then the task search response
        mockSearch
            .mockResolvedValueOnce({entry: [{resource: {id: 'patient-1', resourceType: 'Patient'}}]})
            .mockResolvedValueOnce({
                entry: [{
                    resource: {
                        id: 'task-1',
                        resourceType: 'Task',
                        status: 'ready',
                        meta: {
                            lastUpdated: '2023-01-01T00:00:00Z'
                        },
                        focus: {
                            display: 'Test Service Request'
                        },
                        owner: {
                            display: 'Test Owner'
                        }
                    }
                }]
            });

        await act(async () => {
            render(<TaskOverviewTable/>);
        });

        // First call should be to find the patient
        expect(mockSearch).toHaveBeenNthCalledWith(1, {
            resourceType: 'Patient',
            searchParams: {
                'identifier': 'http://fhir.nl/fhir/NamingSystem/bsn|123456789'
            },
            options: {postSearch: true},
            headers: {
                'Content-Type': 'application/x-www-form-urlencoded'
            }
        });

        // Second call should be to find tasks for the patient
        expect(mockSearch).toHaveBeenNthCalledWith(2, {
            resourceType: 'Task',
            searchParams: {
                'patient': 'Patient/patient-1',
                '_sort': '-_lastUpdated'
            },
            options: {postSearch: true},
            headers: {
                'Content-Type': 'application/x-www-form-urlencoded'
            }
        });
    });

    it('falls back to first available identifier when patient has no BSN', async () => {
        const patientWithoutBSN = {
            id: 'patient-2',
            identifier: [
                {system: 'http://example.com/mrn', value: 'MRN123'}
            ]
        };
        mockSearch
            .mockResolvedValueOnce({entry: [{resource: {id: 'patient-2', resourceType: 'Patient'}}]})
            .mockResolvedValueOnce({entry: []});
        (useEnrollment as jest.Mock).mockReturnValue({patient: patientWithoutBSN});
        (fhirUtils.getPatientIdentifier as jest.Mock).mockReturnValue(null);
        (fhirUtils.identifierToToken as jest.Mock).mockImplementation((identifier) => {
            if (identifier && identifier.system && identifier.value) {
                return `${identifier.system}|${identifier.value}`;
            }
            return '';
        });

        await act(async () => {
            render(<TaskOverviewTable/>);
        });

        expect(mockSearch).toHaveBeenNthCalledWith(1, {
            resourceType: 'Patient',
            searchParams: {
                'identifier': 'http://example.com/mrn|MRN123'
            },
            options: {postSearch: true},
            headers: {
                'Content-Type': 'application/x-www-form-urlencoded'
            }
        });
    });

    it('does not call search when patient is not available', async () => {
        (useEnrollment as jest.Mock).mockReturnValue({patient: null});
        mockSearch.mockResolvedValue([]);

        await act(async () => {
            render(<TaskOverviewTable/>);
        });

        expect(mockSearch).not.toHaveBeenCalled();
    });

    it('throws error when patient has no identifiers', () => {
        const patientWithoutIdentifiers = {id: 'patient-3', identifier: []};
        (useEnrollment as jest.Mock).mockReturnValue({patient: patientWithoutIdentifiers});
        (fhirUtils.getPatientIdentifier as jest.Mock).mockReturnValue(null);

        expect(() => render(<TaskOverviewTable/>)).toThrow('No identifier found for the patient');
    })
});