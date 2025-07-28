import { render, screen } from '@testing-library/react';
import '@testing-library/jest-dom';
import ConfirmDataPreEnrollment from '@/app/enrollment/new/page';

jest.mock('@/app/enrollment/new/components/enroll-in-cps-button', () => {
  return function MockEnrollInCpsButton({ className }: { className?: string }) {
    return <div data-testid="enroll-in-cps-button" className={className}>Enroll Button</div>;
  };
});

jest.mock('@/app/enrollment/new/components/enrollment-details', () => {
  return function MockEnrollmentDetails() {
    return <div data-testid="enrollment-details">Enrollment Details</div>;
  };
});

it('renders the instruction message', () => {
  render(<ConfirmDataPreEnrollment />);
  expect(screen.getByText('Indien het verzoek niet klopt, pas het dan aan in het EPD.')).toBeInTheDocument();
});

it('renders enrollment details component', () => {
  render(<ConfirmDataPreEnrollment />);
  expect(screen.getByTestId('enrollment-details')).toBeInTheDocument();
});

it('renders enroll in cps button component', () => {
  render(<ConfirmDataPreEnrollment />);
  expect(screen.getByTestId('enroll-in-cps-button')).toBeInTheDocument();
});

it('applies mt-5 class to enroll button', () => {
  render(<ConfirmDataPreEnrollment />);
  expect(screen.getByTestId('enroll-in-cps-button')).toHaveClass('mt-5');
});

it('applies correct styling to card container', () => {
  render(<ConfirmDataPreEnrollment />);
  const card = screen.getByText('Indien het verzoek niet klopt, pas het dan aan in het EPD.').closest('[class*="border-0"]');
  expect(card).toHaveClass('border-0', 'shadow-none', 'px-0');
});

it('applies correct styling to card header', () => {
  render(<ConfirmDataPreEnrollment />);
  const cardHeader = screen.getByText('Indien het verzoek niet klopt, pas het dan aan in het EPD.').parentElement;
  expect(screen.getByText('Indien het verzoek niet klopt, pas het dan aan in het EPD.')).toHaveClass('text-muted-foreground');
  expect(cardHeader).toHaveClass('px-0', 'space-y-0', 'pt-0', 'pb-8');
});

it('applies correct styling to card content', () => {
  render(<ConfirmDataPreEnrollment />);
  const cardContent = screen.getByTestId('enrollment-details').parentElement;
  expect(cardContent).toHaveClass('space-y-6', 'px-0');
});

it('renders enroll button outside of card content', () => {
  render(<ConfirmDataPreEnrollment />);
  const enrollButton = screen.getByTestId('enroll-in-cps-button');
  const cardContent = screen.getByTestId('enrollment-details').parentElement;
  expect(cardContent).not.toContainElement(enrollButton);
});
