import '@testing-library/jest-dom'
import { render, screen } from '@testing-library/react'
import EnrollmentLayout from '@/app/enrollment/layout'

describe('EnrollmentLayout', () => {
  it('renders children content within the layout', () => {
    const testContent = 'Test enrollment content'
    
    render(
      <EnrollmentLayout>
        <div>{testContent}</div>
      </EnrollmentLayout>
    )

    expect(screen.getByText(testContent)).toBeInTheDocument()
  })

  it('applies correct wrapper classes', () => {
    render(
      <EnrollmentLayout>
        <div data-testid="child-content">Child content</div>
      </EnrollmentLayout>
    )

    const wrapper = screen.getByTestId('child-content').parentElement
    expect(wrapper).toHaveClass('w-full', 'h-full')
  })

  it('renders multiple children correctly', () => {
    render(
      <EnrollmentLayout>
        <div>First child</div>
        <div>Second child</div>
        <span>Third child</span>
      </EnrollmentLayout>
    )

    expect(screen.getByText('First child')).toBeInTheDocument()
    expect(screen.getByText('Second child')).toBeInTheDocument()
    expect(screen.getByText('Third child')).toBeInTheDocument()
  })

  it('renders with empty children', () => {
    render(<EnrollmentLayout>{null}</EnrollmentLayout>)

    const wrapper = document.querySelector('.w-full.h-full')
    expect(wrapper).toBeInTheDocument()
    expect(wrapper).toBeEmptyDOMElement()
  })

  it('maintains proper DOM structure with nested components', () => {
    render(
      <EnrollmentLayout>
        <div data-testid="parent">
          <div data-testid="nested-child">Nested content</div>
        </div>
      </EnrollmentLayout>
    )

    const parent = screen.getByTestId('parent')
    const nestedChild = screen.getByTestId('nested-child')
    
    expect(parent).toBeInTheDocument()
    expect(nestedChild).toBeInTheDocument()
    expect(parent).toContainElement(nestedChild)
  })

  it('preserves React fragments as children', () => {
    render(
      <EnrollmentLayout>
        <>
          <div>Fragment child 1</div>
          <div>Fragment child 2</div>
        </>
      </EnrollmentLayout>
    )

    expect(screen.getByText('Fragment child 1')).toBeInTheDocument()
    expect(screen.getByText('Fragment child 2')).toBeInTheDocument()
  })

  it('renders with complex component children', () => {
    const ComplexChild = () => (
      <div>
        <h1>Header</h1>
        <p>Paragraph content</p>
        <button>Action button</button>
      </div>
    )

    render(
      <EnrollmentLayout>
        <ComplexChild />
      </EnrollmentLayout>
    )

    expect(screen.getByRole('heading', { level: 1 })).toBeInTheDocument()
    expect(screen.getByText('Paragraph content')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Action button' })).toBeInTheDocument()
  })
})
