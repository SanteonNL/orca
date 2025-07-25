import '@testing-library/jest-dom'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import Error from '../../app/error'
import { getSupportContactLink } from '../../app/actions'

jest.mock('../../app/actions', () => ({
  getSupportContactLink: jest.fn(),
}))

describe('Error', () => {
  const mockReset = jest.fn()
  const mockError = {
    name: 'TestError',
    message: 'Test error message'
  }

  beforeEach(() => {
    jest.clearAllMocks()
    jest.spyOn(Date.prototype, 'toISOString').mockReturnValue('2025-07-24T10:30:00.000Z')
  })

  afterEach(() => {
    jest.restoreAllMocks()
  })

  it('displays error message when provided', () => {
    (getSupportContactLink as jest.Mock).mockResolvedValue(undefined)

    render(<Error error={mockError} reset={mockReset} />)

    expect(screen.getByText('Test error message')).toBeInTheDocument()
  })

  it('displays default message when error message is not provided', () => {
    const errorWithoutMessage = { name: 'TestError', message: '' }
    ;(getSupportContactLink as jest.Mock).mockResolvedValue(undefined)

    render(<Error error={errorWithoutMessage} reset={mockReset} />)

    expect(screen.getByText('Er is een onverwachte fout opgetreden. Probeer het later opnieuw.')).toBeInTheDocument()
  })

  it('calls reset function when retry button is clicked', () => {
    (getSupportContactLink as jest.Mock).mockResolvedValue(undefined)

    render(<Error error={mockError} reset={mockReset} />)

    const retryButton = screen.getByRole('button', { name: /opnieuw proberen/i })
    fireEvent.click(retryButton)

    expect(mockReset).toHaveBeenCalledTimes(1)
  })

  it('displays error digest when provided', () => {
    const errorWithDigest = { ...mockError, digest: 'abc123def456' }
    ;(getSupportContactLink as jest.Mock).mockResolvedValue(undefined)

    render(<Error error={errorWithDigest} reset={mockReset} />)

    expect(screen.getByText('abc123def456')).toBeInTheDocument()
  })

  it('displays default digest when not provided', () => {
    (getSupportContactLink as jest.Mock).mockResolvedValue(undefined)

    render(<Error error={mockError} reset={mockReset} />)

    expect(screen.getByText('000000000')).toBeInTheDocument()
  })

  it('displays current timestamp in ISO format', () => {
    (getSupportContactLink as jest.Mock).mockResolvedValue(undefined)

    render(<Error error={mockError} reset={mockReset} />)

    expect(screen.getByText('2025-07-24T10:30:00.000Z')).toBeInTheDocument()
  })

  it('shows support contact link when available', async () => {
    (getSupportContactLink as jest.Mock).mockResolvedValue('https://support.example.com')

    render(<Error error={mockError} reset={mockReset} />)

    await waitFor(() => {
      const contactLink = screen.getByRole('link', { name: /contact/i })
      expect(contactLink).toBeInTheDocument()
      expect(contactLink).toHaveAttribute('href', 'https://support.example.com')
    })
  })

  it('hides support contact section when link is not available', async () => {
    (getSupportContactLink as jest.Mock).mockResolvedValue(undefined)

    render(<Error error={mockError} reset={mockReset} />)

    await waitFor(() => {
      expect(screen.queryByRole('link', { name: /contact/i })).not.toBeInTheDocument()
    })
  })

  it('hides support contact section when link is empty string', async () => {
    (getSupportContactLink as jest.Mock).mockResolvedValue('')

    render(<Error error={mockError} reset={mockReset} />)

    await waitFor(() => {
      expect(screen.queryByRole('link', { name: /contact/i })).not.toBeInTheDocument()
    })
  })

  it('fetches support contact link on component mount', async () => {
    (getSupportContactLink as jest.Mock).mockResolvedValue('https://support.example.com')

    render(<Error error={mockError} reset={mockReset} />)

    await waitFor(() => {
      expect(getSupportContactLink).toHaveBeenCalledTimes(1)
    })
  })
})
