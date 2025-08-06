import { render, screen } from '@testing-library/react';
import '@testing-library/jest-dom';
import EnrollmentLayout from '@/app/enrollment/layout';
import { usePathname } from 'next/navigation';
import useEnrollmentStore from '@/lib/store/enrollment-store';
import useTaskProgressStore from '@/lib/store/task-progress-store';

jest.mock('next/navigation');
jest.mock('@/lib/store/enrollment-store');
jest.mock('@/lib/store/task-progress-store');

const mockChildren = <div data-testid="children">Test Children</div>;

const mockServiceRequest = {
  code: {
    coding: [{ display: 'cardiologie consult' }]
  }
};

const mockTask = {
  status: 'requested'
};

beforeEach(() => {
  jest.clearAllMocks();
  (usePathname as jest.Mock).mockReturnValue('/enrollment/new');
  (useEnrollmentStore as jest.Mock).mockReturnValue({ serviceRequest: mockServiceRequest });
  (useTaskProgressStore as jest.Mock).mockReturnValue({ task: mockTask });
});

describe('EnrollmentLayout', () => {
  it('renders children content', () => {
    render(<EnrollmentLayout>{mockChildren}</EnrollmentLayout>);
    expect(screen.getByTestId('children')).toBeInTheDocument();
  });

  it('renders horizontal divider line', () => {
    render(<EnrollmentLayout>{mockChildren}</EnrollmentLayout>);
    const divider = document.querySelector('.h-px.bg-gray-200.mb-10');
    expect(divider).toBeInTheDocument();
  });

  it('hides navigation when on overview page', () => {
    (usePathname as jest.Mock).mockReturnValue('/enrollment/list');
    render(<EnrollmentLayout>{mockChildren}</EnrollmentLayout>);
    const nav = screen.getByRole('navigation');
    expect(nav).toBeInTheDocument();
    expect(nav).not.toHaveClass('invisible');
  });

  it('shows navigation with breadcrumb and service when not on overview', () => {
    render(<EnrollmentLayout>{mockChildren}</EnrollmentLayout>);
    const nav = screen.getByRole('navigation');
    expect(nav).toBeInTheDocument();
    expect(screen.getByText('Verzoek controleren')).toBeInTheDocument();
    expect(screen.getByText('cardiologie consult')).toBeInTheDocument();
  });

  it('applies muted text color to service on first step', () => {
    render(<EnrollmentLayout>{mockChildren}</EnrollmentLayout>);
    const service = screen.getByText('cardiologie consult');
    expect(service).toHaveClass('text-muted-foreground');
  });

  it('does not apply muted text color to service on non-first step', () => {
    (useTaskProgressStore as jest.Mock).mockReturnValue({ task: { status: 'accepted' } });
    render(<EnrollmentLayout>{mockChildren}</EnrollmentLayout>);
    const service = screen.getByText('cardiologie consult');
    expect(service).not.toHaveClass('text-muted-foreground');
  });

  it('hides navigation when on last step', () => {
    (useTaskProgressStore as jest.Mock).mockReturnValue({ task: { status: 'accepted' } });
    render(<EnrollmentLayout>{mockChildren}</EnrollmentLayout>);
    const nav = screen.getByRole('navigation');
    expect(nav).toHaveClass('invisible');
  });

  it('shows navigation when not on last step', () => {
    render(<EnrollmentLayout>{mockChildren}</EnrollmentLayout>);
    const nav = screen.getByRole('navigation');
    expect(nav).not.toHaveClass('invisible');
  });

  it('displays status-based title for different task statuses', () => {
    (useTaskProgressStore as jest.Mock).mockReturnValue({ task: { status: 'accepted' } });
    render(<EnrollmentLayout>{mockChildren}</EnrollmentLayout>);
    expect(screen.getByText('Verzoek geaccepteerd')).toBeInTheDocument();
  });

  it('displays service-based title when task status is requested', () => {
    render(<EnrollmentLayout>{mockChildren}</EnrollmentLayout>);
    expect(screen.getByText('cardiologie consult instellen')).toBeInTheDocument();
  });

  it('displays fallback title when no service is available', () => {
    (useEnrollmentStore as jest.Mock).mockReturnValue({ serviceRequest: null });
    render(<EnrollmentLayout>{mockChildren}</EnrollmentLayout>);
    expect(screen.getByText('Instellen')).toBeInTheDocument();
  });

  it('displays overview title when on overview page', () => {
    (usePathname as jest.Mock).mockReturnValue('/enrollment/list');
    (useTaskProgressStore as jest.Mock).mockReturnValue({ task: null });
    render(<EnrollmentLayout>{mockChildren}</EnrollmentLayout>);
    expect(screen.getAllByText('Verzoek controleren').length).toBeGreaterThan(0);
  });


  it('applies first letter uppercase styling to service text', () => {
    render(<EnrollmentLayout>{mockChildren}</EnrollmentLayout>);
    const service = screen.getByText('cardiologie consult');
    expect(service).toHaveClass('first-letter:uppercase');
  });

  it('shows breadcrumb as link when not on first step', () => {
    (useTaskProgressStore as jest.Mock).mockReturnValue({ task: { status: 'accepted' } });
    render(<EnrollmentLayout>{mockChildren}</EnrollmentLayout>);
    const breadcrumbLink = screen.getByRole('link', { name: 'Verzoek controleren' });
    expect(breadcrumbLink).toHaveClass('text-primary', 'font-medium');
  });

  it('shows breadcrumb as span when on first step', () => {
    render(<EnrollmentLayout>{mockChildren}</EnrollmentLayout>);
    const breadcrumbSpan = screen.getByText('Verzoek controleren');
    expect(breadcrumbSpan.tagName.toLowerCase()).toBe('span');
    expect(breadcrumbSpan).toHaveClass('font-medium');
  });
});
