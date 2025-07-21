import { render, screen} from '@testing-library/react'
import '@testing-library/jest-dom'
import QuestionnaireRenderer from '@/app/enrollment/components/questionnaire-renderer'
import useEnrollmentStore from '@/lib/store/enrollment-store'
import useCpsClient from '@/hooks/use-cps-client'
import * as fhirUtils from '@/lib/fhirUtils'
import * as populateUtils from '../../../../app/utils/populate'


jest.mock('@aehrc/smart-forms-renderer', () => ({
  useQuestionnaireResponseStore: {
    use: {
      updatableResponse: jest.fn(),
      responseIsValid: jest.fn()
    },
    setState: jest.fn()
  },
  useRendererQueryClient: jest.fn(() => ({ mount: jest.fn() })),
  useBuildForm: jest.fn(() => false),
  BaseRenderer: () => <div data-testid="base-renderer" />,
}))
jest.mock('@/lib/store/enrollment-store')
jest.mock('@/hooks/use-cps-client')
jest.mock('@/lib/fhirUtils')
jest.mock('../../../../app/utils/populate')
jest.mock('@tanstack/react-query', () => ({
  ...jest.requireActual('@tanstack/react-query'),
  QueryClientProvider: jest.fn(({ children }) => <div data-testid="mock-query-client-provider">{children}</div>),
  useQueryClient: jest.fn(() => ({
    mount: jest.fn(),
    unmount: jest.fn(),
  })),
}))

const mockQuestionnaire = { id: 'q1', resourceType: 'Questionnaire' as const, status: 'active' as const, item: [
    { linkId: '1', text: 'Question 1', type: 'string' as const, code: [
        {
          code: "763077003",
          system: "http://snomed.info/sct",
          display: "Asthma Control Questionnaire score"
        }
      ],
      "required": true, },
    { linkId: '2', text: 'Question 2', type: 'boolean' as const, code: [
        {
          code: "763077003",
          system: "http://snomed.info/sct",
          display: "Asthma Control Questionnaire score"
        }
      ],
      "required": true, }
  ] }
//const mockTask = { id: 't1', resourceType: 'Task' as const, intent: 'order', status: 'requested', output: [] }
const mockPatient = { id: 'p1' }
const mockPractitioner = { id: 'pr1' }
//const mockUpdatableResponse = { id: 'qr1' }


beforeEach(() => {
  jest.restoreAllMocks();
  (useEnrollmentStore as jest.Mock).mockReturnValue({ patient: mockPatient, practitioner: mockPractitioner });
  (useCpsClient as jest.Mock).mockReturnValue({ transaction: jest.fn().mockResolvedValue({}) });
  (fhirUtils.findQuestionnaireResponse as jest.Mock).mockResolvedValue(undefined);
  (populateUtils.populateQuestionnaire as jest.Mock).mockResolvedValue({ populateResult: { populated: { id: 'populated' } } });
})

it('renders loading state when questionnaire is not provided', () => {
  render(<QuestionnaireRenderer questionnaire={undefined as any} />)
  const loadingElement = screen.getByTitle('loading-spinner')
  expect(loadingElement).toBeInTheDocument()
})


it('renders first page with the button verzoek versturen', async () => {
  render(<QuestionnaireRenderer questionnaire={mockQuestionnaire} />)
  const button = await screen.findByRole('button', { name: /verzoek versturen/i })
  expect(button).not.toBeNull()
})
//
// it('disables submit button when response is invalid', () => {
//   useQuestionnaireResponseStore.use.responseIsValid.mockReturnValue(false)
//   render(<QuestionnaireRenderer questionnaire={mockQuestionnaire} inputTask={mockTask} />)
//   const button = screen.getByRole('button', { name: /verzoek versturen/i })
//   expect(button).toBeDisabled()
// })
//
// it('shows error toast if inputTask or updatableResponse is missing on submit', async () => {
//   useQuestionnaireResponseStore.use.updatableResponse.mockReturnValue(undefined)
//   render(<QuestionnaireRenderer questionnaire={mockQuestionnaire} inputTask={undefined as any} />)
//   const button = screen.getByRole('button', { name: /verzoek versturen/i })
//   fireEvent.click(button)
//   await waitFor(() => {
//     expect(screen.getByText(/cannot set questionnaireresponse/i)).toBeInTheDocument()
//   })
// })
//
// it('prepopulates questionnaire response when not already prepopulated', async () => {
//   render(<QuestionnaireRenderer questionnaire={mockQuestionnaire} inputTask={mockTask} />)
//   await waitFor(() => {
//     expect(populateUtils.populateQuestionnaire).toHaveBeenCalled()
//     expect(useQuestionnaireResponseStore.setState).toHaveBeenCalledWith({ updatableResponse: { id: 'populated' } })
//   })
// })
//
// it('does not prepopulate if patient or practitioner is missing', () => {
//   useEnrollmentStore.mockReturnValue({ patient: null, practitioner: null })
//   render(<QuestionnaireRenderer questionnaire={mockQuestionnaire} inputTask={mockTask} />)
//   expect(populateUtils.populateQuestionnaire).not.toHaveBeenCalled()
// })
