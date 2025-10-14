import { render, screen } from '@testing-library/react';
import '@testing-library/jest-dom';
import ConfirmDataPreEnrollment from '@/app/enrollment/new/page';
import useEnrollment from "@/app/hooks/enrollment-hook";

jest.mock('@/app/hooks/enrollment-hook');

const mockServiceRequest = {
    performer: [{identifier: {system: 'http://example.com', value: 'org-123'}}]
};
(useEnrollment as jest.Mock).mockReturnValue({
    serviceRequest: mockServiceRequest
});

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
  expect(screen.getByText('Indien de gegevens van de patiënt niet kloppen, pas het dan aan in het EPD.')).toBeInTheDocument();
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
