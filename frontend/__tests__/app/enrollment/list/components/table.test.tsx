import {act, render, screen} from '@testing-library/react';
import '@testing-library/jest-dom';
import TaskOverviewTable from '@/app/enrollment/list/components/table';
import useEnrollment from '@/app/hooks/enrollment-hook';
import useContext from '@/app/hooks/context-hook';
import * as fhirUtils from '@/lib/fhirUtils';

jest.mock('@/app/hooks/enrollment-hook');
jest.mock('@/app/hooks/context-hook');
jest.mock('@/lib/fhirUtils');

const mockPatient = {
    id: 'patient-1',
    identifier: [
        {system: 'http://fhir.nl/fhir/NamingSystem/bsn', value: '123456789'}
    ]
};

beforeEach(() => {
    jest.clearAllMocks();
    (useEnrollment as jest.Mock).mockReturnValue({patient: mockPatient});
    const mockSearchFn = jest.fn();
    mockSearchFn.mockResolvedValue([]);
    (useContext as jest.Mock).mockReturnValue({
        launchContext: {taskIdentifier: 'task-id-123'},
        cpsClient: {search: mockSearchFn}
    });
    (fhirUtils.getPatientIdentifier as jest.Mock).mockReturnValue(mockPatient.identifier[0]);
});

describe('TaskOverviewTable', () => {
    it('renders table headers correctly', async () => {
        await act(async () => {
            render(<TaskOverviewTable/>);
        });
        expect(screen.getByText('Uitvoerder')).toBeInTheDocument();
        expect(screen.getByText('Verzoek')).toBeInTheDocument();
        expect(screen.getByText('Status')).toBeInTheDocument();
        expect(screen.getByText('Datum')).toBeInTheDocument();
    });

    it('calls cpsClient search with correct patient identifier when patient has BSN', async () => {
        const mockSearch = jest.fn();
        mockSearch.mockResolvedValue([]);
        (useContext as jest.Mock).mockReturnValue({
            launchContext: {taskIdentifier: 'task-id-123'},
            cpsClient: {search: mockSearch}
        });

        await act(async () => {
            render(<TaskOverviewTable/>);
        });

        expect(mockSearch).toHaveBeenCalledWith({
            resourceType: 'Task',
            searchParams: {
                'patient': 'http://fhir.nl/fhir/NamingSystem/bsn|123456789'
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
        const mockSearch = jest.fn();
        mockSearch.mockResolvedValue([]);
        (useContext as jest.Mock).mockReturnValue({
            launchContext: {taskIdentifier: 'task-id-123'},
            cpsClient: {search: mockSearch}
        });
        (useEnrollment as jest.Mock).mockReturnValue({patient: patientWithoutBSN});
        (fhirUtils.getPatientIdentifier as jest.Mock).mockReturnValue(null);

        await act(async () => {
            render(<TaskOverviewTable/>);
        });

        expect(mockSearch).toHaveBeenCalledWith({
            resourceType: 'Task',
            searchParams: {
                'patient': 'http://example.com/mrn|MRN123'
            }
        });
    });

    it('does not call search when patient is not available', async () => {
        (useEnrollment as jest.Mock).mockReturnValue({patient: null});
        const mockSearch = jest.fn();
        mockSearch.mockResolvedValue([]);
        (useContext as jest.Mock).mockReturnValue({
            launchContext: {taskIdentifier: 'task-id-123'},
            cpsClient: {search: mockSearch}
        });

        await act(async () => {
            render(<TaskOverviewTable/>);
        });

        expect(mockSearch).not.toHaveBeenCalled();
    });

    it('throws error when patient has no identifiers', () => {
        const patientWithoutIdentifiers = {id: 'patient-3', identifier: []};
        (useEnrollment as jest.Mock).mockReturnValue({patient: patientWithoutIdentifiers});
        (fhirUtils.getPatientIdentifier as jest.Mock).mockReturnValue(null);

        expect(() => render(<TaskOverviewTable/>)).toThrow('No patient identifier found for the patient');
    })
});