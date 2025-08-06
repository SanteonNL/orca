import {act, render, screen} from '@testing-library/react';
import '@testing-library/jest-dom';
import TaskOverviewTable from '@/app/enrollment/list/components/table';
import useEnrollmentStore from '@/lib/store/enrollment-store';
import useContext from '@/lib/store/context-store';
import * as fhirUtils from '@/lib/fhirUtils';
import EnrollmentTaskPage from "@/app/enrollment/task/[taskId]/page";

jest.mock('@/lib/store/enrollment-store');
jest.mock('@/lib/store/context-store');
jest.mock('@/lib/fhirUtils');

const mockPatient = {
    id: 'patient-1',
    identifier: [
        {system: 'http://fhir.nl/fhir/NamingSystem/bsn', value: '123456789'}
    ]
};

const mockTasks = [
    {
        id: 'task-1',
        owner: {display: 'Dr. Smith'},
        focus: {display: 'Cardiologie consult'},
        status: 'requested',
        lastModified: '2024-01-15'
    },
    {
        id: 'task-2',
        owner: {display: 'Dr. Johnson'},
        focus: {display: 'Bloedonderzoek'},
        status: 'completed',
        lastModified: '2024-01-14'
    }
];

beforeEach(() => {
    jest.clearAllMocks();
    (useEnrollmentStore as jest.Mock).mockReturnValue({patient: mockPatient});
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
        (useEnrollmentStore as jest.Mock).mockReturnValue({patient: patientWithoutBSN});
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
        (useEnrollmentStore as jest.Mock).mockReturnValue({patient: null});
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
        (useEnrollmentStore as jest.Mock).mockReturnValue({patient: patientWithoutIdentifiers});
        (fhirUtils.getPatientIdentifier as jest.Mock).mockReturnValue(null);

        expect(() => render(<TaskOverviewTable/>)).toThrow('No patient identifier found for the patient');
    })
});