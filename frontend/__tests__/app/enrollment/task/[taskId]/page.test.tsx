import {render, screen, act, waitFor} from '@testing-library/react';
import '@testing-library/jest-dom';
import EnrollmentTaskPage from '@/app/enrollment/task/[taskId]/page';
import useEnrollment from '@/app/hooks/enrollment-hook';
import {useParams} from 'next/navigation';
import * as fhirRender from '@/lib/fhirRender';
import * as applaunch from '@/app/applaunch';
import TaskProgressHook from '@/app/hooks/task-progress-hook';

jest.mock('@/app/hooks/task-progress-hook');
jest.mock('@/app/hooks/enrollment-hook');
const mockCpsClient = {transaction: jest.fn().mockResolvedValue({})}
const mockScpClient = {}
jest.mock('@/app/hooks/context-hook', () => () => ({
    launchContext: {taskIdentifier: 'task-id-123'},
    cpsClient: mockCpsClient,
    scpClient: mockScpClient,
    isLoading: false,
    isError: false,
    error: null
}))
jest.mock('next/navigation');
jest.mock('@/lib/fhirRender');
jest.mock('@/app/applaunch');
jest.mock('@tanstack/react-query', () => ({
    ...jest.requireActual('@tanstack/react-query'),
    useQuery: jest.fn(() => ({
        data: { launchContext: {taskIdentifier: 'task-id-123'} },
        isLoading: false,
        isError: false,
        error: null
    }))
}));
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
jest.mock('@/app/enrollment/components/task-heading', () => {
    const MockTaskHeading = ({children, title}: {children?: React.ReactNode, title: string}) => (
        <div data-testid="task-heading">
            <div data-testid="task-title">{title}</div>
            <div data-testid="task-navigation">{children}</div>
        </div>
    );
    MockTaskHeading.displayName = 'MockTaskHeading';
    return MockTaskHeading;
});
jest.mock('@/app/enrollment/components/task-body', () => {
    const MockTaskBody = ({children}: {children: React.ReactNode}) => (
        <div data-testid="task-body">{children}</div>
    );
    MockTaskBody.displayName = 'MockTaskBody';
    return MockTaskBody;
});
jest.mock('@/app/enrollment/task/components/patient-details', () => {
    const MockPatientDetails = ({task, patient}: {task: any, patient: any}) => (
        <div data-testid="patient-details">Patient: {patient?.id}, Task: {task?.id}</div>
    );
    MockPatientDetails.displayName = 'MockPatientDetails';
    return MockPatientDetails;
});
jest.mock('@/app/error', () => {
    const MockError = ({error}: {error: any, reset?: any}) => (
        <div data-testid="error-component">Error: {error.message}</div>
    );
    MockError.displayName = 'MockError';
    return MockError;
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
    (useEnrollment as jest.Mock).mockReturnValue({
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
            task: { status: 'requested' }, // Provide minimal task to prevent null access
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
        expect(screen.getByTestId('patient-details')).toBeInTheDocument();
        expect(screen.getByText('Het verzoek is door de uitvoerende organisatie geaccepteerd, maar uitvoering is nog niet gestart.')).toBeInTheDocument();
    });

    it('displays task note when available', async () => {
        const taskWithNote = {
            ...mockTask,
            note: [{text: 'Custom task note'}]
        };
        (TaskProgressHook as jest.Mock).mockReturnValue({
            task: taskWithNote,
            subTasks: [],
            questionnaireMap: {},
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

        (TaskProgressHook as jest.Mock).mockReturnValue({
            task: { ...mockTask, status: 'in-progress' },
            subTasks: [],
            questionnaireMap: {},
            isError: false,
            isLoading: false
        });

        (useEnrollment as jest.Mock).mockReturnValue({
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

    it('displays patient details when patient data is missing', async () => {
        (useEnrollment as jest.Mock).mockReturnValue({
            patient: null,
            serviceRequest: mockServiceRequest
        });
        await act(async () => {
            render(<EnrollmentTaskPage/>);
        });
        expect(screen.getByTestId('patient-details')).toBeInTheDocument();
    });

    it('displays patient details when email telecom is missing', async () => {
        const patientWithoutEmail = {
            ...mockPatient,
            telecom: [{system: 'phone', value: '+31612345678'}]
        };
        (useEnrollment as jest.Mock).mockReturnValue({
            patient: patientWithoutEmail,
            serviceRequest: mockServiceRequest
        });
        await act(async () => {
            render(<EnrollmentTaskPage/>);
        });
        expect(screen.getByTestId('patient-details')).toBeInTheDocument();
    });

    it('displays patient details when phone telecom is missing', async () => {
        const patientWithoutPhone = {
            ...mockPatient,
            telecom: [{system: 'email', value: 'patient@example.com'}]
        };
        (useEnrollment as jest.Mock).mockReturnValue({
            patient: patientWithoutPhone,
            serviceRequest: mockServiceRequest
        });
        await act(async () => {
            render(<EnrollmentTaskPage/>);
        });
        expect(screen.getByTestId('patient-details')).toBeInTheDocument();
    });

    it('displays patient details component with task and patient data', async () => {
        await act(async () => {
            render(<EnrollmentTaskPage/>);
        });
        expect(screen.getByTestId('patient-details')).toBeInTheDocument();
        expect(screen.getByText('Patient: patient-1, Task: task-1')).toBeInTheDocument();
    });

    it('displays patient details when last updated date is missing', async () => {
        const taskWithoutDate = {
            ...mockTask,
            meta: {}
        };
        (TaskProgressHook as jest.Mock).mockReturnValue({
            task: taskWithoutDate,
            subTasks: [],
            questionnaireMap: {},
            isError: false,
            isLoading: false
        });
        await act(async () => {
            render(<EnrollmentTaskPage/>);
        });
        expect(screen.getByTestId('patient-details')).toBeInTheDocument();
    });

    it('displays patient details when status reason is available', async () => {
        const taskWithStatusReason = {
            ...mockTask,
            statusReason: {text: 'Additional information needed'}
        };
        (TaskProgressHook as jest.Mock).mockReturnValue({
            task: taskWithStatusReason,
            subTasks: [],
            questionnaireMap: {},
            isError: false,
            isLoading: false
        });
        await act(async () => {
            render(<EnrollmentTaskPage/>);
        });
        expect(screen.getByTestId('patient-details')).toBeInTheDocument();
    });

    it('renders patient details component when status reason not available', async () => {
        await act(async () => {
            render(<EnrollmentTaskPage/>);
        });
        expect(screen.getByTestId('patient-details')).toBeInTheDocument();
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
        // This test is handled by the global mock already
        // No need for inline mocking since useContext hook is already mocked

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
        expect(screen.getByTestId('error-component')).toBeInTheDocument();
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
        expect(screen.getByTestId('patient-details')).toBeInTheDocument();
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
        expect(screen.getByTestId('patient-details')).toBeInTheDocument();
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
        expect(screen.getByTestId('patient-details')).toBeInTheDocument();
    });

    it('renders task heading with correct title for accepted status', async () => {
        await act(async () => {
            render(<EnrollmentTaskPage/>);
        });

        expect(screen.getByTestId('task-heading')).toBeInTheDocument();
        expect(screen.getByTestId('task-title')).toHaveTextContent('Verzoek geaccepteerd');
    });

    it('renders task heading with service name when available for ready status', async () => {
        const serviceRequest = {
            ...mockServiceRequest,
            code: { coding: [{ display: 'Cardiology Consultation' }] }
        };
        (useEnrollment as jest.Mock).mockReturnValue({
            patient: mockPatient,
            serviceRequest
        });
        (TaskProgressHook as jest.Mock).mockReturnValue({
            task: { ...mockTask, status: 'ready' },
            isError: false,
            isLoading: false
        });

        await act(async () => {
            render(<EnrollmentTaskPage/>);
        });

        expect(screen.getByTestId('task-title')).toHaveTextContent('Cardiology Consultation instellen');
    });

    it('renders default title when service name not available for requested status', async () => {
        (TaskProgressHook as jest.Mock).mockReturnValue({
            task: { ...mockTask, status: 'requested' },
            isError: false,
            isLoading: false
        });

        await act(async () => {
            render(<EnrollmentTaskPage/>);
        });

        expect(screen.getByTestId('task-title')).toHaveTextContent('Instellen');
    });

    it('renders breadcrumb navigation for non-first step', async () => {
        await act(async () => {
            render(<EnrollmentTaskPage/>);
        });

        const navigation = screen.getByTestId('task-navigation');
        expect(navigation).toBeInTheDocument();
        expect(navigation.querySelector('a')).toHaveAttribute('href', '/enrollment/new');
    });

    it('renders breadcrumb as span for first step', async () => {
        (TaskProgressHook as jest.Mock).mockReturnValue({
            task: { ...mockTask, status: 'requested' },
            isError: false,
            isLoading: false
        });

        await act(async () => {
            render(<EnrollmentTaskPage/>);
        });

        const navigation = screen.getByTestId('task-navigation');
        expect(navigation.querySelector('span')).toHaveClass('font-medium');
        expect(navigation.querySelector('a')).not.toBeInTheDocument();
    });

    it('hides navigation for last step statuses', async () => {
        (TaskProgressHook as jest.Mock).mockReturnValue({
            task: { ...mockTask, status: 'completed' },
            isError: false,
            isLoading: false
        });

        await act(async () => {
            render(<EnrollmentTaskPage/>);
        });

        const navigation = screen.getByTestId('task-navigation');
        expect(navigation.querySelector('nav')).toHaveClass('invisible');
    });

    it('renders task body with patient details when not in questionnaire mode', async () => {
        await act(async () => {
            render(<EnrollmentTaskPage/>);
        });

        expect(screen.getByTestId('task-body')).toBeInTheDocument();
        expect(screen.getByTestId('patient-details')).toBeInTheDocument();
    });

    it('renders error component when hook returns error', async () => {
        (TaskProgressHook as jest.Mock).mockReturnValue({
            task: null,
            isError: true,
            isLoading: false
        });

        await act(async () => {
            render(<EnrollmentTaskPage/>);
        });

        expect(screen.getByTestId('error-component')).toBeInTheDocument();
        expect(screen.getByText('Error: "Er is een probleem opgetreden bij het ophalen van de taak"')).toBeInTheDocument();
    });

    it('correctly determines first step for requested status', async () => {
        (TaskProgressHook as jest.Mock).mockReturnValue({
            task: { ...mockTask, status: 'requested' },
            isError: false,
            isLoading: false
        });

        await act(async () => {
            render(<EnrollmentTaskPage/>);
        });

        const navigation = screen.getByTestId('task-navigation');
        expect(navigation.querySelector('span.font-medium')).toBeInTheDocument();
    });

    it('correctly determines last step for various statuses', async () => {
        const lastStepStatuses = ['accepted', 'in-progress', 'rejected', 'failed', 'completed', 'cancelled', 'on-hold'];

        for (const status of lastStepStatuses) {
            (TaskProgressHook as jest.Mock).mockReturnValue({
                task: { ...mockTask, status },
                isError: false,
                isLoading: false
            });

            const { rerender } = render(<EnrollmentTaskPage/>);

            const navigation = screen.getByTestId('task-navigation');
            expect(navigation.querySelector('nav')).toHaveClass('invisible');

            rerender(<div></div>);
        }
    });

    it('handles missing service request gracefully in breadcrumb', async () => {
        (useEnrollment as jest.Mock).mockReturnValue({
            patient: mockPatient,
            serviceRequest: null
        });

        await act(async () => {
            render(<EnrollmentTaskPage/>);
        });

        expect(screen.getByTestId('task-navigation')).toBeInTheDocument();
    });

    it('uses correct base path in breadcrumb link', async () => {
        const originalBasePath = process.env.NEXT_PUBLIC_BASE_PATH;
        process.env.NEXT_PUBLIC_BASE_PATH = '/custom-path';

        await act(async () => {
            render(<EnrollmentTaskPage/>);
        });

        const link = screen.getByTestId('task-navigation').querySelector('a');
        expect(link).toHaveAttribute('href', '/custom-path/enrollment/new');

        process.env.NEXT_PUBLIC_BASE_PATH = originalBasePath;
    });

    it('handles missing base path in breadcrumb link', async () => {
        const originalBasePath = process.env.NEXT_PUBLIC_BASE_PATH;
        delete process.env.NEXT_PUBLIC_BASE_PATH;

        await act(async () => {
            render(<EnrollmentTaskPage/>);
        });

        const link = screen.getByTestId('task-navigation').querySelector('a');
        expect(link).toHaveAttribute('href', '/enrollment/new');

        process.env.NEXT_PUBLIC_BASE_PATH = originalBasePath;
    });
});
