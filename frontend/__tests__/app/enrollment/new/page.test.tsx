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

it('renders enroll button outside of card content', () => {
  render(<ConfirmDataPreEnrollment />);
  const enrollButton = screen.getByTestId('enroll-in-cps-button');
  const cardContent = screen.getByTestId('enrollment-details').parentElement;
  expect(cardContent).not.toContainElement(enrollButton);
});
