import {render, screen, act, waitFor} from '@testing-library/react';
import '@testing-library/jest-dom';
import EnrollmentTaskPage from '@/app/enrollment/task/[taskId]/page';
import useEnrollmentStore from '@/lib/store/enrollment-store';
import {useParams} from 'next/navigation';
import * as fhirRender from '@/lib/fhirRender';
import * as applaunch from '@/app/applaunch';
import TaskProgressHook from '@/app/hooks/task-progress-hook';

jest.mock('@/app/hooks/task-progress-hook');
jest.mock('@/lib/store/enrollment-store');
const mockCpsClient = {transaction: jest.fn().mockResolvedValue({})}
const mockScpClient = {}
jest.mock('@/lib/store/context-store', () => ({
    useContextStore: jest.fn(() => ({
        launchContext: {taskIdentifier: 'task-id-123'},
        cpsClient: mockCpsClient,
        scpClient: mockScpClient
    }))
}))
jest.mock('next/navigation');
jest.mock('@/lib/fhirRender');
jest.mock('@/app/applaunch');
jest.mock('@/app/enrollment/loading', () => {
    const MockLoading = () => <div data-testid="loading">Loading...</div>;
    MockLoading.displayName = 'MockLoading';
    return MockLoading;
});
jest.mock('@/app/enrollment/components/questionnaire-renderer', () => {
    const MockQuestionnaireRenderer = ({questionnaire, inputTask}: {questionnaire?: any, inputTask?: any}) => (
        <div data-testid="questionnaire-renderer">Questionnaire: {questionnaire?.id}, Task: {inputTask?.id}</div>
    );
    MockQuestionnaireRenderer.displayName = 'MockQuestionnaireRenderer';
    return MockQuestionnaireRenderer;
});

const mockTask = {
    id: 'task-1',
    status: 'accepted',
    focus: {display: 'Cardiologie consult'},
    reasonCode: {coding: [{display: 'Heart condition'}]},
    owner: {reference: 'Organization/org-1'},
    meta: {lastUpdated: '2024-01-15T10:00:00Z'},
    note: []
};

const mockPatient = {
    id: 'patient-1',
    telecom: [
        {system: 'email', value: 'patient@example.com'},
        {system: 'phone', value: '+31612345678'}
    ]
};

const mockServiceRequest = {
    performer: [{identifier: {system: 'http://example.com', value: 'org-123'}}]
};

const mockOrganization = {
    id: 'org-1',
    name: 'Test Hospital'
};

beforeEach(() => {
    jest.clearAllMocks();
    (useParams as jest.Mock).mockReturnValue({taskId: 'task-1'});
    (TaskProgressHook as jest.Mock).mockReturnValue({
        task: mockTask,
        subTasks: [],
        questionnaireMap: {},
        isError: false,
        isLoading: false
    });
    (useEnrollmentStore as jest.Mock).mockReturnValue({
        patient: mockPatient,
        serviceRequest: mockServiceRequest
    });
    (fhirRender.patientName as jest.Mock).mockReturnValue('John Doe');
    (fhirRender.organizationName as jest.Mock).mockReturnValue('Test Hospital');
    (applaunch.getLaunchableApps as jest.Mock).mockResolvedValue([]);
});

describe("taskid page tests", () => {
    it('displays loading component when store is loading', async () => {
        (TaskProgressHook as jest.Mock).mockReturnValue({
            isLoading: true,
            task: null,
            subTasks: [],
            questionnaireMap: {},
            isError: false
        });
        await act(async () => {
            render(<EnrollmentTaskPage/>);
        });
        expect(screen.getByTestId('loading')).toBeInTheDocument();
    });


    it('displays task not found message when task is null', async () => {
        (TaskProgressHook as jest.Mock).mockReturnValue({
            task: null,
            isLoading: false,
            isError: false
        });
        await act(async () => {
            render(<EnrollmentTaskPage/>);
        });
        expect(screen.getByText('Taak niet gevonden')).toBeInTheDocument();
    });


    it('renders questionnaire when task status is received and questionnaire available', async () => {
        const mockQuestionnaire = {id: 'questionnaire-1'};
        const mockSubTask = {id: 'subtask-1'};
        (TaskProgressHook as jest.Mock).mockReturnValue({
            task: {...mockTask, status: 'received'},
            subTasks: [mockSubTask],
            questionnaireMap: {'subtask-1': mockQuestionnaire},
            isError: false,
            isLoading: false
        });
        await act(async () => {
            render(<EnrollmentTaskPage/>);
        });
        expect(screen.getByTestId('questionnaire-renderer')).toBeInTheDocument();
        expect(screen.getByText('Questionnaire: questionnaire-1, Task: subtask-1')).toBeInTheDocument();
    });

    it('displays task status information when not in questionnaire mode', async () => {
        await act(async () => {
            render(<EnrollmentTaskPage/>);
        });
        expect(screen.getByText('Patiënt:')).toBeInTheDocument();
        expect(screen.getByText('John Doe')).toBeInTheDocument();
        expect(screen.getByText('E-mailadres:')).toBeInTheDocument();
        expect(screen.getByText('patient@example.com')).toBeInTheDocument();
        expect(screen.getByText('Telefoonnummer:')).toBeInTheDocument();
        expect(screen.getByText('+31612345678')).toBeInTheDocument();
    });

    it('displays task note when available', async () => {
        const taskWithNote = {
            ...mockTask,
            note: [{text: 'Custom task note'}]
        };
        (TaskProgressHook as jest.Mock).mockReturnValue({
            task: taskWithNote,
            isError: false,
            isLoading: false
        });
        await act(async () => {
            render(<EnrollmentTaskPage/>);
        });
        expect(screen.getByText('Custom task note')).toBeInTheDocument();
    });

    it('displays execution text when no task note available', async () => {
        await act(async () => {
            render(<EnrollmentTaskPage/>);
        });
        expect(screen.getByText('Het verzoek is door de uitvoerende organisatie geaccepteerd, maar uitvoering is nog niet gestart.')).toBeInTheDocument();
    });

    it('displays launch buttons when task is in-progress and apps available', async () => {
        const mockApps = [
            {Name: 'App 1', URL: 'http://app1.example.com'},
            {Name: 'App 2', URL: 'http://app2.example.com'}
        ];
        (applaunch.getLaunchableApps as jest.Mock).mockResolvedValue(mockApps);

        // Mock the task progress store with in-progress status and autoLaunch disabled
        (TaskProgressHook as jest.Mock).mockReturnValue({
            task: { ...mockTask, status: 'in-progress' },
            isError: false,
            isLoading: false
        });

        // Mock enrollment store with proper data
        (useEnrollmentStore as jest.Mock).mockReturnValue({
            patient: mockPatient,
            serviceRequest: mockServiceRequest,
            organization: mockOrganization
        });

        await act(async () => {
            render(<EnrollmentTaskPage/>);
        });

        await waitFor(() => {
            expect(screen.getByText('App 1')).toBeInTheDocument();
            expect(screen.getByText('App 2')).toBeInTheDocument();
        });
    });

    it('displays onbekend when patient data is missing', async () => {
        (useEnrollmentStore as jest.Mock).mockReturnValue({
            patient: null,
            serviceRequest: mockServiceRequest
        });
        await act(async () => {
            render(<EnrollmentTaskPage/>);
        });
        const patientRow = screen.getByText('Patiënt:').nextElementSibling;
        expect(patientRow).toHaveTextContent('Onbekend');
    });

    it('displays onbekend when email telecom is missing', async () => {
        const patientWithoutEmail = {
            ...mockPatient,
            telecom: [{system: 'phone', value: '+31612345678'}]
        };
        (useEnrollmentStore as jest.Mock).mockReturnValue({
            patient: patientWithoutEmail,
            serviceRequest: mockServiceRequest
        });
        await act(async () => {
            render(<EnrollmentTaskPage/>);
        });
        const emailRow = screen.getByText('E-mailadres:').nextElementSibling;
        expect(emailRow).toHaveTextContent('Onbekend');
    });

    it('displays onbekend when phone telecom is missing', async () => {
        const patientWithoutPhone = {
            ...mockPatient,
            telecom: [{system: 'email', value: 'patient@example.com'}]
        };
        (useEnrollmentStore as jest.Mock).mockReturnValue({
            patient: patientWithoutPhone,
            serviceRequest: mockServiceRequest
        });
        await act(async () => {
            render(<EnrollmentTaskPage/>);
        });
        const phoneRow = screen.getByText('Telefoonnummer:').nextElementSibling;
        expect(phoneRow).toHaveTextContent('Onbekend');
    });

    it('displays formatted last updated date in status', async () => {
        await act(async () => {
            render(<EnrollmentTaskPage/>);
        });
        expect(screen.getByText(/Geaccepteerd op 15-1-2024/)).toBeInTheDocument();
    });

    it('displays onbekend when last updated date is missing', async () => {
        const taskWithoutDate = {
            ...mockTask,
            meta: {}
        };
        (TaskProgressHook as jest.Mock).mockReturnValue({
            task: taskWithoutDate,
            isError: false,
            isLoading: false
        });
        await act(async () => {
            render(<EnrollmentTaskPage/>);
        });
        expect(screen.getByText(/Geaccepteerd op Onbekend/)).toBeInTheDocument();
    });

    it('displays status reason when available', async () => {
        const taskWithStatusReason = {
            ...mockTask,
            statusReason: {text: 'Additional information needed'}
        };
        (TaskProgressHook as jest.Mock).mockReturnValue({
            task: taskWithStatusReason,
            isError: false,
            isLoading: false
        });
        await act(async () => {
            render(<EnrollmentTaskPage/>);
        });
        expect(screen.getByText('Statusreden:')).toBeInTheDocument();
        expect(screen.getByText('Additional information needed')).toBeInTheDocument();
    });

    it('does not display status reason section when not available', async () => {
        await act(async () => {
            render(<EnrollmentTaskPage/>);
        });
        expect(screen.queryByText('Statusreden:')).not.toBeInTheDocument();
    });

    it('does not render launch buttons when auto launch is disabled and task is not accepted', async () => {
        const taskInProgress = {...mockTask, status: 'in-progress'};
        (TaskProgressHook as jest.Mock).mockReturnValue({
            task: taskInProgress,
            isError: false,
            isLoading: false
        });
        await act(async () => {
            render(<EnrollmentTaskPage/>);
        });
        expect(screen.queryByRole('button')).not.toBeInTheDocument();
    });

    it('passes correct parameters to TaskProgressHook', async () => {
        (useParams as jest.Mock).mockReturnValue({taskId: 'test-task-123'});

        await act(async () => {
            render(<EnrollmentTaskPage/>);
        });

        expect(TaskProgressHook).toHaveBeenCalledWith({
            taskId: 'test-task-123',
            cpsClient: mockCpsClient,
            pollingInterval: 1000
        });
    });

    it('handles array taskId parameter correctly in hook call', async () => {
        (useParams as jest.Mock).mockReturnValue({taskId: ['array-task-1', 'array-task-2']});

        await act(async () => {
            render(<EnrollmentTaskPage/>);
        });

        expect(TaskProgressHook).toHaveBeenCalledWith({
            taskId: 'array-task-1',
            cpsClient: mockCpsClient,
            pollingInterval: 1000
        });
    });

    it('handles missing cpsClient gracefully', async () => {
        jest.mock('@/lib/store/context-store', () => ({
            useContextStore: jest.fn(() => ({
                scpClient: mockScpClient,
                cpsClient: null
            }))
        }));

        (TaskProgressHook as jest.Mock).mockReturnValue({
            task: null,
            isError: false,
            isLoading: true
        });

        await act(async () => {
            render(<EnrollmentTaskPage/>);
        });

        expect(screen.getByTestId('loading')).toBeInTheDocument();
    });

    it('auto launches external app when conditions are met', async () => {
        const mockApps = [{Name: 'External App', URL: 'http://external.example.com'}];
        const originalEnv = process.env.NEXT_PUBLIC_AUTOLAUNCH_EXTERNAL_APP;
        process.env.NEXT_PUBLIC_AUTOLAUNCH_EXTERNAL_APP = 'true';
        const mockOpen = jest.fn();
        Object.defineProperty(window, 'open', { value: mockOpen, writable: true });

        (applaunch.getLaunchableApps as jest.Mock).mockResolvedValue(mockApps);
        (TaskProgressHook as jest.Mock).mockReturnValue({
            task: { ...mockTask, status: 'in-progress' },
            isError: false,
            isLoading: false
        });

        await act(async () => {
            render(<EnrollmentTaskPage/>);
        });

        await waitFor(() => {
            expect(mockOpen).toHaveBeenCalledWith('http://external.example.com', '_self');
        });

        process.env.NEXT_PUBLIC_AUTOLAUNCH_EXTERNAL_APP = originalEnv;
    });

    it('does not auto launch when multiple apps are available', async () => {
        const mockApps = [
            {Name: 'App 1', URL: 'http://app1.example.com'},
            {Name: 'App 2', URL: 'http://app2.example.com'}
        ];
        const originalEnv = process.env.NEXT_PUBLIC_AUTOLAUNCH_EXTERNAL_APP;
        process.env.NEXT_PUBLIC_AUTOLAUNCH_EXTERNAL_APP = 'true';
        const mockOpen = jest.fn();
        Object.defineProperty(window, 'open', { value: mockOpen, writable: true });

        (applaunch.getLaunchableApps as jest.Mock).mockResolvedValue(mockApps);
        (TaskProgressHook as jest.Mock).mockReturnValue({
            task: { ...mockTask, status: 'in-progress' },
            isError: false,
            isLoading: false
        });

        await act(async () => {
            render(<EnrollmentTaskPage/>);
        });

        expect(mockOpen).not.toHaveBeenCalled();

        process.env.NEXT_PUBLIC_AUTOLAUNCH_EXTERNAL_APP = originalEnv;
    });

    it('does not auto launch when task is not in progress', async () => {
        const mockApps = [{Name: 'External App', URL: 'http://external.example.com'}];
        const originalEnv = process.env.NEXT_PUBLIC_AUTOLAUNCH_EXTERNAL_APP;
        process.env.NEXT_PUBLIC_AUTOLAUNCH_EXTERNAL_APP = 'true';
        const mockOpen = jest.fn();
        Object.defineProperty(window, 'open', { value: mockOpen, writable: true });

        (applaunch.getLaunchableApps as jest.Mock).mockResolvedValue(mockApps);
        (TaskProgressHook as jest.Mock).mockReturnValue({
            task: { ...mockTask, status: 'accepted' },
            isError: false,
            isLoading: false
        });

        await act(async () => {
            render(<EnrollmentTaskPage/>);
        });

        expect(mockOpen).not.toHaveBeenCalled();

        process.env.NEXT_PUBLIC_AUTOLAUNCH_EXTERNAL_APP = originalEnv;
    });

    it('handles hook error state', async () => {
        (TaskProgressHook as jest.Mock).mockReturnValue({
            task: null,
            isError: true,
            isLoading: false
        });

        await act(async () => {
            render(<EnrollmentTaskPage/>);
        });

        expect(screen.getByText('Taak niet gevonden')).toBeInTheDocument();
    });

    it('displays questionnaire only when all conditions are met', async () => {
        const mockQuestionnaire = {id: 'questionnaire-1'};
        const mockSubTask = {id: 'subtask-1'};

        (TaskProgressHook as jest.Mock).mockReturnValue({
            task: {...mockTask, status: 'received'},
            subTasks: [mockSubTask],
            questionnaireMap: {'subtask-1': mockQuestionnaire},
            isError: false,
            isLoading: false
        });

        await act(async () => {
            render(<EnrollmentTaskPage/>);
        });

        expect(screen.getByTestId('questionnaire-renderer')).toBeInTheDocument();
        expect(screen.queryByText('Patiënt:')).not.toBeInTheDocument();
    });

    it('does not display questionnaire when task status is not received', async () => {
        const mockQuestionnaire = {id: 'questionnaire-1'};
        const mockSubTask = {id: 'subtask-1'};

        (TaskProgressHook as jest.Mock).mockReturnValue({
            task: {...mockTask, status: 'accepted'},
            subTasks: [mockSubTask],
            questionnaireMap: {'subtask-1': mockQuestionnaire},
            isError: false,
            isLoading: false
        });

        await act(async () => {
            render(<EnrollmentTaskPage/>);
        });

        expect(screen.queryByTestId('questionnaire-renderer')).not.toBeInTheDocument();
        expect(screen.getByText('Patiënt:')).toBeInTheDocument();
    });

    it('does not display questionnaire when no subtasks available', async () => {
        const mockQuestionnaire = {id: 'questionnaire-1'};

        (TaskProgressHook as jest.Mock).mockReturnValue({
            task: {...mockTask, status: 'received'},
            subTasks: [],
            questionnaireMap: {'subtask-1': mockQuestionnaire},
            isError: false,
            isLoading: false
        });

        await act(async () => {
            render(<EnrollmentTaskPage/>);
        });

        expect(screen.queryByTestId('questionnaire-renderer')).not.toBeInTheDocument();
        expect(screen.getByText('Patiënt:')).toBeInTheDocument();
    });

    it('does not display questionnaire when questionnaire map is empty', async () => {
        const mockSubTask = {id: 'subtask-1'};

        (TaskProgressHook as jest.Mock).mockReturnValue({
            task: {...mockTask, status: 'received'},
            subTasks: [mockSubTask],
            questionnaireMap: {},
            isError: false,
            isLoading: false
        });

        await act(async () => {
            render(<EnrollmentTaskPage/>);
        });

        expect(screen.queryByTestId('questionnaire-renderer')).not.toBeInTheDocument();
        expect(screen.getByText('Patiënt:')).toBeInTheDocument();
    });
});
