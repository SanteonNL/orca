import {act, fireEvent, render, screen, waitFor} from '@testing-library/react'
import '@testing-library/jest-dom'
import QuestionnaireRenderer from '@/app/enrollment/components/questionnaire-renderer'
import useEnrollment from '@/app/hooks/enrollment-hook'
import * as fhirUtils from '@/lib/fhirUtils'
import * as populateUtils from '../../../../app/utils/populate'
import {useQuestionnaireResponseStore} from "@aehrc/smart-forms-renderer";
import {toast} from 'sonner'



const mockQuestionnaire = {
    id: 'q1', resourceType: 'Questionnaire' as const, status: 'active' as const, item: [
        {
            linkId: '1', text: 'Question 1', type: 'string' as const, code: [
                {
                    code: "763077003",
                    system: "http://snomed.info/sct",
                    display: "Asthma Control Questionnaire score"
                }
            ],
            "required": true,
        },
        {
            linkId: '2', text: 'Question 2', type: 'boolean' as const, code: [
                {
                    code: "763077003",
                    system: "http://snomed.info/sct",
                    display: "Asthma Control Questionnaire score"
                }
            ],
            "required": true,
        }
    ]
}
const mockUpdatableResponse = {
    id: 'qr1',
    resourceType: 'QuestionnaireResponse' as const,
    status: 'in-progress' as const,
    questionnaire: mockQuestionnaire.id,
    item: []
}
const mockTask = {
    id: 't1',
    resourceType: 'Task' as const,
    intent: 'order' as const,
    status: 'requested' as const,
    output: []
}
const mockPatient = {id: 'p1'}
const mockPractitioner = {id: 'pr1'}
const mockQuestionnaireResponse = {
    id: 'qr1',
    resourceType: 'QuestionnaireResponse' as const,
    status: 'in-progress' as const,
    questionnaire: mockQuestionnaire.id,
    item: []
}


const mockResponseIsValid = jest.fn();
jest.mock('@aehrc/smart-forms-renderer', () => ({
    useQuestionnaireResponseStore: {
        use: {
            updatableResponse: jest.fn().mockImplementation(() => mockUpdatableResponse),
            responseIsValid: jest.fn().mockImplementation(() => mockResponseIsValid())
        },
        setState: jest.fn()
    },
    useRendererQueryClient: jest.fn(() => ({mount: jest.fn()})),
    useBuildForm: jest.fn(() => false),
    BaseRenderer: () => <div data-testid="base-renderer"/>,
}))

jest.mock('sonner', () => ({toast: {error: jest.fn(), success: jest.fn(),}}))
jest.mock('@/app/hooks/enrollment-hook')

const mockCpsClient = {transaction: jest.fn().mockResolvedValue({})}
jest.mock('@/lib/store/context-store', () => ({
    useContextStore: jest.fn(() => ({
        launchContext: {taskIdentifier: 'task-id-123'},
        cpsClient: mockCpsClient
    }))
}))
jest.mock('@/lib/fhirUtils')
jest.mock('../../../../app/utils/populate')
jest.mock('@tanstack/react-query', () => ({
    ...jest.requireActual('@tanstack/react-query'),
    QueryClientProvider: jest.fn(({children}) => <div data-testid="mock-query-client-provider">{children}</div>),
    useQueryClient: jest.fn(() => ({
        mount: jest.fn(),
        unmount: jest.fn(),
    })),
}))


beforeEach(() => {
    jest.restoreAllMocks();
    mockResponseIsValid.mockReturnValue(true);
    (useEnrollment as jest.Mock).mockReturnValue({patient: mockPatient, practitioner: mockPractitioner});
    (fhirUtils.findQuestionnaireResponse as jest.Mock).mockResolvedValue(mockQuestionnaireResponse);
    (populateUtils.populateQuestionnaire as jest.Mock).mockResolvedValue({populateResult: {populated: {id: 'populated'}}});
})
describe("QuestionnaireRenderer", () => {
    it('renders loading state when questionnaire is not provided', () => {
        render(<QuestionnaireRenderer questionnaire={undefined as any}/>)
        const loadingElement = screen.getByTitle('loading-spinner')
        expect(loadingElement).toBeInTheDocument()
    })


    it('renders and submits questionnaire response successfully', async () => {
        render(<QuestionnaireRenderer questionnaire={mockQuestionnaire} inputTask={mockTask}/>)
        const button = await screen.findByRole('button', {name: /verzoek versturen/i})
        expect(button).not.toBeNull()
        fireEvent.click(button!)
        await waitFor(() => {
            expect(mockCpsClient.transaction).toHaveBeenCalled()
        })
    })
//
    it('disables submit button when response is invalid', async () => {
        mockResponseIsValid.mockReturnValue(false)
        await act(async () => {
            render(<QuestionnaireRenderer questionnaire={mockQuestionnaire} inputTask={mockTask}/>)
        });


        const button = screen.getByRole('button', {name: /verzoek versturen/i})
        expect(button).toBeDisabled()
    })

    it('shows error toast if inputTask or updatableResponse is missing on submit', async () => {
        (useQuestionnaireResponseStore.use.updatableResponse as jest.Mock).mockReturnValue(undefined)
        await act(async () => {
            render(<QuestionnaireRenderer questionnaire={mockQuestionnaire} inputTask={undefined as any}/>)
        });

        const button = screen.getByRole('button', {name: /verzoek versturen/i})
        fireEvent.click(button)
        await waitFor(() => {
            expect(toast.error).toHaveBeenCalled()
        })
    })

    it('prepopulates questionnaire response when not already prepopulated', async () => {
        await act(async () => {
            render(<QuestionnaireRenderer questionnaire={mockQuestionnaire} inputTask={mockTask}/>)
        });

        await waitFor(() => {
            expect(populateUtils.populateQuestionnaire).toHaveBeenCalled()
            expect(useQuestionnaireResponseStore.setState).toHaveBeenCalledWith({updatableResponse: {id: 'populated'}})
        })
    })

    it('does not prepopulate if patient or practitioner is missing', async () => {
        (useEnrollment as jest.Mock).mockReturnValue({patient: null, practitioner: null})
        await act(async () => {
            render(<QuestionnaireRenderer questionnaire={mockQuestionnaire} inputTask={mockTask}/>)
        });

        expect(populateUtils.populateQuestionnaire).not.toHaveBeenCalled()
    })
});
