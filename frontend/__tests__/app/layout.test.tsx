import '@testing-library/jest-dom'
import { render, screen, waitFor } from '@testing-library/react'
import { toast } from 'sonner'
import RootLayout, { metadata } from '../../app/layout'

beforeAll(() => {
  window.matchMedia = window.matchMedia || function() {
    return {
      matches: false,
      media: '',
      onchange: null,
      addListener: jest.fn(),
      removeListener: jest.fn(),
      addEventListener: jest.fn(),
      removeEventListener: jest.fn(),
      dispatchEvent: jest.fn(),
    };
  };
});

describe('RootLayout', () => {
  it('renders children correctly', () => {
    const testContent = <div data-testid="test-content">Test Content</div>

    render(<RootLayout>{testContent}</RootLayout>)

    // Check if content is rendered
    expect(screen.getByTestId('test-content')).toBeInTheDocument()
    expect(screen.getByText('Test Content')).toBeInTheDocument()
  })

  it('contains main element', () => {
    render(<RootLayout><div>Test</div></RootLayout>)

    // Check if main element exists
    const mainElement = screen.getByRole('main')
    expect(mainElement).toBeInTheDocument()
  })

  it('includes Toaster component', async () => {
    render(<RootLayout><div>Test</div></RootLayout>)

    // Trigger a toast to ensure the Toaster component renders
    toast('Test toast')

    await waitFor(() => {
      // Query the global document for the Toaster element (handles portal rendering)
      expect(document.body.querySelector('.toaster, [id="sonner-toaster"], [data-sonner-toaster]')).not.toBeNull()
    })
  })
})

describe('metadata', () => {
  it('has correct default title', () => {
    expect(metadata.title).toBe('ORCA Frontend')
  })

  it('has correct description', () => {
    expect(metadata.description).toBe('This app renders Questionnaires based on the SDC specification. It allows users to input required data before a Task is published to placer(s)')
  })
})
