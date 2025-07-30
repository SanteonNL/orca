import {render, screen, fireEvent, waitFor} from '@testing-library/react';
import '@testing-library/jest-dom';
import EnrollInCpsButton from '@/app/enrollment/new/components/enroll-in-cps-button';
import useEnrollment from '@/lib/store/enrollment-store';
import useCpsClient from '@/hooks/use-cps-client';
import * as fhirUtils from '@/lib/fhirUtils';
import {useRouter} from 'next/navigation';
import {toast} from 'sonner';

jest.mock('@/lib/store/enrollment-store');
jest.mock('@/hooks/use-cps-client');
jest.mock('@/lib/fhirUtils');
jest.mock('next/navigation');
jest.mock('sonner');

const mockPatient = {
    id: 'patient-1',
    identifier: [{system: 'http://fhir.nl/fhir/NamingSystem/bsn', value: '123456789'}]
};
const mockTaskCondition = {id: 'condition-1', resourceType: 'Condition'};
const mockPractitionerRole = {id: 'practitioner-role-1'};
const mockServiceRequest = {id: 'service-request-1'};
const mockTaskBundle = {resourceType: 'Bundle', type: 'transaction', entry: []};
const mockTask = {id: 'task-1', resourceType: 'Task'};

beforeEach(() => {
    jest.clearAllMocks();
    (useEnrollment as jest.Mock).mockReturnValue({
        patient: mockPatient,
        selectedCarePlan: {id: 'care-plan-1'},
        taskCondition: mockTaskCondition,
        practitionerRole: mockPractitionerRole,
        serviceRequest: mockServiceRequest,
        loading: false,
        launchContext: {taskIdentifier: 'task-id-123'}
    });
    (useCpsClient as jest.Mock).mockReturnValue({transaction: jest.fn().mockResolvedValue(mockTaskBundle)});
    (fhirUtils.getPatientIdentifier as jest.Mock).mockReturnValue(mockPatient.identifier[0]);
    (fhirUtils.constructTaskBundle as jest.Mock).mockReturnValue(mockTaskBundle);
    (fhirUtils.findInBundle as jest.Mock).mockReturnValue(mockTask);
    (useRouter as jest.Mock).mockReturnValue({push: jest.fn()});
});

describe("enroll in cps button test", () => {

    it('renders button with correct text and icon', () => {
        render(<EnrollInCpsButton/>);
        expect(screen.getByRole('button', {name: /volgende stap/i})).toBeInTheDocument();
    });

    it('button is enabled when all required data is present', () => {
        render(<EnrollInCpsButton/>);
        expect(screen.getByRole('button')).not.toBeDisabled();
    });

    it('button is disabled when taskCondition is missing', () => {
        (useEnrollment as jest.Mock).mockReturnValue({
            patient: mockPatient,
            taskCondition: null,
            loading: false
        });
        render(<EnrollInCpsButton/>);
        expect(screen.getByRole('button')).toBeDisabled();
    });

    it('button is disabled when loading is true', () => {
        (useEnrollment as jest.Mock).mockReturnValue({
            patient: mockPatient,
            taskCondition: mockTaskCondition,
            loading: true
        });
        render(<EnrollInCpsButton/>);
        expect(screen.getByRole('button')).toBeDisabled();
    });

    it('creates task and navigates to task page on successful submission', async () => {
        const mockPush = jest.fn();
        (useRouter as jest.Mock).mockReturnValue({push: mockPush});

        render(<EnrollInCpsButton/>);

        fireEvent.click(screen.getByRole('button'));

        await waitFor(() => {
            expect(mockPush).toHaveBeenCalledWith('/enrollment/task/task-1');
        });
    });

    it('shows spinner when submission is in progress', async () => {
        render(<EnrollInCpsButton/>);

        fireEvent.click(screen.getByRole('button'));

        expect(screen.getByRole('button')).toBeDisabled();
    });


    it('handles missing cpsClient error', async () => {
        (useCpsClient as jest.Mock).mockReturnValue(null);

        render(<EnrollInCpsButton/>);

        fireEvent.click(screen.getByRole('button'));

        await waitFor(() => {
            expect(screen.getByRole('button')).not.toBeDisabled();
        });
    });

    it('handles missing patient data error', async () => {
        (useEnrollment as jest.Mock).mockReturnValue({
            patient: null,
            taskCondition: mockTaskCondition,
            practitionerRole: mockPractitionerRole,
            serviceRequest: mockServiceRequest,
            loading: false
        });

        render(<EnrollInCpsButton/>);

        fireEvent.click(screen.getByRole('button'));

        await waitFor(() => {
            expect(screen.getByRole('button')).not.toBeDisabled();
        });
    });

    it('handles task bundle construction error', async () => {
        (fhirUtils.constructTaskBundle as jest.Mock).mockImplementation(() => {
            throw new Error('Bundle construction failed');
        });

        render(<EnrollInCpsButton/>);

        fireEvent.click(screen.getByRole('button'));

        await waitFor(() => {
            expect(toast.error).toHaveBeenCalled();
            expect(screen.getByRole('button')).not.toBeDisabled();
        });
    });

    it('handles cps transaction error', async () => {
        const mockTransaction = jest.fn().mockRejectedValue(new Error('Transaction failed'));
        (useCpsClient as jest.Mock).mockReturnValue({transaction: mockTransaction});

        render(<EnrollInCpsButton/>);

        fireEvent.click(screen.getByRole('button'));

        await waitFor(() => {
            expect(screen.getByRole('button')).not.toBeDisabled();
        });
    });

    it('displays validation errors for 400 status response', async () => {
        const validationError = {
            response: {
                status: 400,
                data: {
                    issue: [
                        {diagnostics: 'Patient identifier is required'},
                        {diagnostics: 'Invalid condition code'}
                    ]
                }
            }
        };
        const mockTransaction = jest.fn().mockRejectedValue(validationError);
        (useCpsClient as jest.Mock).mockReturnValue({transaction: mockTransaction});

        render(<EnrollInCpsButton/>);

        fireEvent.click(screen.getByRole('button'));

        await waitFor(() => {
            expect(screen.getByText('Validation Errors:')).toBeInTheDocument();
            expect(screen.getByText('Patient identifier is required')).toBeInTheDocument();
            expect(screen.getByText('Invalid condition code')).toBeInTheDocument();
        });
    });

    it('displays unknown error message when validation errors have no diagnostics', async () => {
        const validationError = {
            response: {
                status: 400,
                data: {
                    issue: [
                        {code: 'required'},
                        {code: 'invalid'}
                    ]
                }
            }
        };
        const mockTransaction = jest.fn().mockRejectedValue(validationError);
        (useCpsClient as jest.Mock).mockReturnValue({transaction: mockTransaction});

        render(<EnrollInCpsButton/>);

        fireEvent.click(screen.getByRole('button'));

        await waitFor(() => {
            expect(screen.getByText('An unknown error occurred')).toBeInTheDocument();
        });
    });

    it('handles missing task in bundle response', async () => {
        (fhirUtils.findInBundle as jest.Mock).mockReturnValue(null);

        render(<EnrollInCpsButton/>);

        fireEvent.click(screen.getByRole('button'));

        await waitFor(() => {
            expect(screen.getByRole('button')).not.toBeDisabled();
        });
    });
});