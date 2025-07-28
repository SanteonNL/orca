import {render, screen, act, waitFor} from '@testing-library/react';
import '@testing-library/jest-dom';
import EnrollmentTaskPage from '@/app/enrollment/task/[taskId]/page';
import useTaskProgressStore from '@/lib/store/task-progress-store';
import useEnrollmentStore from '@/lib/store/enrollment-store';
import {useParams} from 'next/navigation';
import * as fhirRender from '@/lib/fhirRender';
import * as applaunch from '@/app/applaunch';

jest.mock('@/lib/store/task-progress-store');
jest.mock('@/lib/store/enrollment-store');
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
jest.mock('@/app/enrollment/components/sse-connection-status', () => {
    const MockSseConnectionStatus = () => <div data-testid="sse-status">SSE Status</div>;
    MockSseConnectionStatus.displayName = 'MockSseConnectionStatus';
    return MockSseConnectionStatus;
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
    (useTaskProgressStore as jest.Mock).mockReturnValue({
        task: mockTask,
        loading: false,
        initialized: true,
        setSelectedTaskId: jest.fn(),
        subTasks: [],
        taskToQuestionnaireMap: {}
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
        (useTaskProgressStore as jest.Mock).mockReturnValue({
            loading: true,
            initialized: false,
            setSelectedTaskId: jest.fn(),
            subTasks: [],
            taskToQuestionnaireMap: {}
        });
        await act(async () => {
            render(<EnrollmentTaskPage/>);
        });
        expect(screen.getByTestId('loading')).toBeInTheDocument();
    });

    it('displays loading component when store is not initialized', async () => {
        (useTaskProgressStore as jest.Mock).mockReturnValue({
            loading: false,
            initialized: false,
            setSelectedTaskId: jest.fn(),
            subTasks: [],
            taskToQuestionnaireMap: {}
        });
        await act(async () => {
            render(<EnrollmentTaskPage/>);
        });
        expect(screen.getByTestId('loading')).toBeInTheDocument();
    });

    it('displays task not found message when task is null', async () => {
        (useTaskProgressStore as jest.Mock).mockReturnValue({
            task: null,
            loading: false,
            initialized: true,
            setSelectedTaskId: jest.fn(),
            subTasks: [],
            taskToQuestionnaireMap: {}
        });
        await act(async () => {
            render(<EnrollmentTaskPage/>);
        });
        expect(screen.getByText('Taak niet gevonden')).toBeInTheDocument();
    });

    it('sets selected task id from url params on mount', async () => {
        const mockSetSelectedTaskId = jest.fn();
        (useTaskProgressStore as jest.Mock).mockReturnValue({
            task: mockTask,
            loading: false,
            initialized: true,
            setSelectedTaskId: mockSetSelectedTaskId,
            subTasks: [],
            taskToQuestionnaireMap: {}
        });
        await act(async () => {
            render(<EnrollmentTaskPage/>);
        });
        expect(mockSetSelectedTaskId).toHaveBeenCalledWith('task-1');
    });

    it('handles array task id by taking first element', async () => {
        const mockSetSelectedTaskId = jest.fn();
        (useParams as jest.Mock).mockReturnValue({taskId: ['task-1', 'task-2']});
        (useTaskProgressStore as jest.Mock).mockReturnValue({
            task: mockTask,
            loading: false,
            initialized: true,
            setSelectedTaskId: mockSetSelectedTaskId,
            subTasks: [],
            taskToQuestionnaireMap: {}
        });
        await act(async () => {
            render(<EnrollmentTaskPage/>);
        });
        expect(mockSetSelectedTaskId).toHaveBeenCalledWith('task-1');
    });

    it('renders questionnaire when task status is received and questionnaire available', async () => {
        const mockQuestionnaire = {id: 'questionnaire-1'};
        const mockSubTask = {id: 'subtask-1'};
        (useTaskProgressStore as jest.Mock).mockReturnValue({
            task: {...mockTask, status: 'received'},
            loading: false,
            initialized: true,
            setSelectedTaskId: jest.fn(),
            subTasks: [mockSubTask],
            taskToQuestionnaireMap: {'subtask-1': mockQuestionnaire}
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
        (useTaskProgressStore as jest.Mock).mockReturnValue({
            task: taskWithNote,
            loading: false,
            initialized: true,
            setSelectedTaskId: jest.fn(),
            subTasks: [],
            taskToQuestionnaireMap: {}
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

    it('displays launch buttons when task is accepted and apps available', async () => {
        const mockApps = [
            {Name: 'App 1', URL: 'http://app1.example.com'},
            {Name: 'App 2', URL: 'http://app2.example.com'}
        ];
        (applaunch.getLaunchableApps as jest.Mock).mockResolvedValue(mockApps);

        // Mock the task progress store with in-progress status and autoLaunch disabled
        (useTaskProgressStore as jest.Mock).mockReturnValue({
            task: { ...mockTask, status: 'in-progress' },
            loading: false,
            initialized: true,
            setSelectedTaskId: jest.fn(),
            subTasks: [],
            taskToQuestionnaireMap: {},
            autoLaunchExternalApps: false
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
        (useTaskProgressStore as jest.Mock).mockReturnValue({
            task: taskWithoutDate,
            loading: false,
            initialized: true,
            setSelectedTaskId: jest.fn(),
            subTasks: [],
            taskToQuestionnaireMap: {}
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
        (useTaskProgressStore as jest.Mock).mockReturnValue({
            task: taskWithStatusReason,
            loading: false,
            initialized: true,
            setSelectedTaskId: jest.fn(),
            subTasks: [],
            taskToQuestionnaireMap: {}
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

    it('renders sse connection status component', async () => {
        await act(async () => {
            render(<EnrollmentTaskPage/>);
        });
        expect(screen.getByTestId('sse-status')).toBeInTheDocument();
    });

    it('does not render launch buttons when auto launch is disabled and task is not accepted', async () => {
        const taskInProgress = {...mockTask, status: 'in-progress'};
        (useTaskProgressStore as jest.Mock).mockReturnValue({
            task: taskInProgress,
            loading: false,
            initialized: true,
            setSelectedTaskId: jest.fn(),
            subTasks: [],
            taskToQuestionnaireMap: {}
        });
        await act(async () => {
            render(<EnrollmentTaskPage/>);
        });
        expect(screen.queryByRole('button')).not.toBeInTheDocument();
    })
});
