import '@testing-library/jest-dom'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import Error from '../../app/error'
import { getSupportContactEmail } from '../../app/actions'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { useMemo } from 'react'
import { ErrorWithTitle } from '@/app/utils/error-with-title'

jest.mock('../../app/actions', () => ({
  getSupportContactEmail: jest.fn(),
}))

const wrapper = ({
  children
}: {
  children: React.ReactNode
}) => {
  const client = useMemo(() => new QueryClient(), [])
  return <QueryClientProvider client={client}>{children}</QueryClientProvider>
}

describe('Error', () => {
  const mockReset = jest.fn()
  const mockError = new ErrorWithTitle('Test Error', 'Test error message')

  beforeEach(() => {
    jest.clearAllMocks()
    jest.spyOn(Date.prototype, 'toISOString').mockReturnValue('2025-07-24T10:30:00.000Z')
  })

  afterEach(() => {
    jest.restoreAllMocks()
  })

  it('displays error message when provided', () => {
    (getSupportContactEmail as jest.Mock).mockResolvedValue(undefined)

    render(<Error error={mockError} reset={mockReset} />, { wrapper })

    expect(screen.getByText('Test error message')).toBeInTheDocument()
  })

  it('displays default message when error message is not provided', () => {
    const errorWithoutMessage = { name: 'TestError', message: '' }
    ;(getSupportContactEmail as jest.Mock).mockResolvedValue(undefined)

    render(<Error error={errorWithoutMessage} reset={mockReset} />, { wrapper })

    expect(screen.getByText('We konden dit scherm niet laden. Probeer het alsjeblieft opnieuw.')).toBeInTheDocument()
  })

  it('shows support contact link when available', async () => {
    (getSupportContactEmail as jest.Mock).mockResolvedValue('https://support.example.com')

    render(<Error error={mockError} reset={mockReset} />, { wrapper })

    await waitFor(() => {
      expect(screen.getByText('https://support.example.com')).toBeInTheDocument()
    })
  })

  it('hides support contact section when link is not available', async () => {
    (getSupportContactEmail as jest.Mock).mockResolvedValue(undefined)

    render(<Error error={mockError} reset={mockReset} />, { wrapper })

    await waitFor(() => {
      expect(screen.queryByRole('link', { name: /contact/i })).not.toBeInTheDocument()
    })
  })

  it('hides support contact section when link is empty string', async () => {
    (getSupportContactEmail as jest.Mock).mockResolvedValue('')

    render(<Error error={mockError} reset={mockReset} />, { wrapper })

    await waitFor(() => {
      expect(screen.queryByRole('link', { name: /contact/i })).not.toBeInTheDocument()
    })
  })

  it('fetches support contact link on component mount', async () => {
    (getSupportContactEmail as jest.Mock).mockResolvedValue('https://support.example.com')

    render(<Error error={mockError} reset={mockReset} />, { wrapper })

    await waitFor(() => {
      expect(getSupportContactEmail).toHaveBeenCalledTimes(1)
    })
  })
})
