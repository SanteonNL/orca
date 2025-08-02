import { render, screen } from '@testing-library/react';
import '@testing-library/jest-dom';
import { Coding } from 'fhir/r4';
import ValidationErrors from '@/app/enrollment/new/components/validation-errors';

describe('ValidationErrors', () => {
  it('displays notification banner with correct styling', () => {
    const validationErrors: Coding[] = [{ code: 'E0001' }];

    const { container } = render(<ValidationErrors validationErrors={validationErrors} />);

    const banner = container.firstChild as HTMLElement;
    expect(banner).toHaveClass('border', 'border-red-500', 'rounded-lg', 'p-4', 'bg-red-50', 'mb-4');
  });

  it('shows alert icon and error title', () => {
    const validationErrors: Coding[] = [{ code: 'E0001' }];

    render(<ValidationErrors validationErrors={validationErrors} />);

    expect(screen.getByText('Er gaat iets mis')).toBeInTheDocument();
  });

  it('displays message for missing email error', () => {
    const validationErrors: Coding[] = [{ code: 'E0001' }];

    render(<ValidationErrors validationErrors={validationErrors} />);

    expect(screen.getByText('Er gaat iets mis')).toBeInTheDocument();
    expect(screen.getByText(/Er is geen e-mailadres/)).toBeInTheDocument();
  });

  it('displays message for missing phone error', () => {
    const validationErrors: Coding[] = [{ code: 'E0002' }];

    render(<ValidationErrors validationErrors={validationErrors} />);

    expect(screen.getByText(/Er is geen telefoonnummer/)).toBeInTheDocument();
  });

  it('displays message for invalid email error', () => {
    const validationErrors: Coding[] = [{ code: 'E0003' }];

    render(<ValidationErrors validationErrors={validationErrors} />);

    expect(screen.getByText(/Controleer het e-mailadres/)).toBeInTheDocument();
  });

  it('displays message for invalid phone error', () => {
    const validationErrors: Coding[] = [{ code: 'E0004' }];

    render(<ValidationErrors validationErrors={validationErrors} />);

    expect(screen.getByText(/Controleer het telefoonnummer/)).toBeInTheDocument();
  });

  it('displays combined message for missing email and phone', () => {
    const validationErrors: Coding[] = [{ code: 'E0001' }, { code: 'E0002' }];

    render(<ValidationErrors validationErrors={validationErrors} />);

    expect(screen.getByText(/Er zijn geen e-mailadres en telefoonnummer/)).toBeInTheDocument();
  });

  it('displays combined message for invalid email and phone', () => {
    const validationErrors: Coding[] = [{ code: 'E0003' }, { code: 'E0004' }];

    render(<ValidationErrors validationErrors={validationErrors} />);

    expect(screen.getByText(/Controleer het e-mailadres en telefoonnummer/)).toBeInTheDocument();
  });

  it('displays separate messages for mixed error types', () => {
    const validationErrors: Coding[] = [{ code: 'E0001' }, { code: 'E0004' }];

    render(<ValidationErrors validationErrors={validationErrors} />);

    expect(screen.getByText(/Er is geen e-mailadres/)).toBeInTheDocument();
    expect(screen.getByText(/Controleer het telefoonnummer/)).toBeInTheDocument();
  });

  it('displays multiple paragraphs for individual errors', () => {
    const validationErrors: Coding[] = [{ code: 'E0001' }, { code: 'E0004' }];

    const { container } = render(<ValidationErrors validationErrors={validationErrors} />);

    const paragraphs = container.querySelectorAll('p');
    expect(paragraphs).toHaveLength(2);
  });

  it('displays single paragraph for combined errors', () => {
    const validationErrors: Coding[] = [{ code: 'E0001' }, { code: 'E0002' }];

    const { container } = render(<ValidationErrors validationErrors={validationErrors} />);

    const paragraphs = container.querySelectorAll('p');
    expect(paragraphs).toHaveLength(1);
  });

  it('displays unknown error message for empty validation errors', () => {
    const validationErrors: Coding[] = [];

    render(<ValidationErrors validationErrors={validationErrors} />);

    expect(screen.getByText(/Er is een onbekende fout opgetreden/)).toBeInTheDocument();
  });

  it('displays unknown error message for unrecognized error codes', () => {
    const validationErrors: Coding[] = [{ code: 'E9999' }];

    render(<ValidationErrors validationErrors={validationErrors} />);

    expect(screen.getByText(/Er is een onbekende fout opgetreden/)).toBeInTheDocument();
  });

  it('handles validation errors without code property', () => {
    const validationErrors: Coding[] = [{}];

    render(<ValidationErrors validationErrors={validationErrors} />);

    expect(screen.getByText(/Er is een onbekende fout opgetreden/)).toBeInTheDocument();
  });

  it('handles mixed valid and invalid error codes', () => {
    const validationErrors: Coding[] = [{ code: 'E0001' }, { code: 'INVALID' }];

    render(<ValidationErrors validationErrors={validationErrors} />);

    expect(screen.getByText(/Er is geen e-mailadres/)).toBeInTheDocument();
  });
});
