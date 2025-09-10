import {render, screen} from '@testing-library/react';
import '@testing-library/jest-dom';
import EnrollmentDetails from '@/app/enrollment/new/components/enrollment-details';
import useEnrollment from '@/app/hooks/enrollment-hook';
import * as fhirRender from '@/lib/fhirRender';

jest.mock('@/app/hooks/enrollment-hook');
jest.mock('@/lib/fhirRender');

const mockPatient = {
    id: 'patient-1',
    name: [{given: ['John'], family: 'Doe'}],
    telecom: [
        {system: 'email', value: 'john.doe@example.com'},
        {system: 'phone', value: '+31612345678'}
    ]
};

const mockServiceRequest = {
    id: 'service-request-1',
    code: {
        coding: [{display: 'cardiologie consult'}]
    },
    performer: [
        {
            reference: 'Organization/org-1',
            display: 'Test Hospital',
        }
    ]
};

const mockTaskCondition = {
    id: 'condition-1',
    code: {
        text: 'hartfalen',
        coding: [{display: 'heart failure'}]
    }
};

beforeEach(() => {
    jest.clearAllMocks();
    (useEnrollment as jest.Mock).mockReturnValue({
        patient: mockPatient,
        serviceRequest: mockServiceRequest,
        taskCondition: mockTaskCondition,
        isLoading: false
    });
    (fhirRender.patientName as jest.Mock).mockReturnValue('John Doe');
    (fhirRender.organizationName as jest.Mock).mockReturnValue('Test Hospital');
    (fhirRender.organizationNameShort as jest.Mock).mockReturnValue('Test Hospital');
});
describe("EnrollmentDetails component", () => {
    it('displays spinner when loading is true', () => {
        (useEnrollment as jest.Mock).mockReturnValue({isLoading: true});
        render(<EnrollmentDetails/>);
        expect(document.querySelector('.text-primary')).toBeInTheDocument();
    });

    it('renders patient name when patient data is available', () => {
        render(<EnrollmentDetails/>);
        expect(screen.getByText('John Doe')).toBeInTheDocument();
    });

    it('displays onbekend when patient is null', () => {
        (useEnrollment as jest.Mock).mockReturnValue({
            patient: null,
            serviceRequest: mockServiceRequest,
            taskCondition: mockTaskCondition,
            isLoading: false
        });
        render(<EnrollmentDetails/>);
        const patientRow = screen.getByText('Patiënt:').nextElementSibling;
        expect(patientRow).toHaveTextContent('Onbekend');
    });

    it('displays email address when telecom email is available', () => {
        render(<EnrollmentDetails/>);
        expect(screen.getByText('john.doe@example.com')).toBeInTheDocument();
    });

    it('displays onbekend when patient has no email telecom', () => {
        const patientWithoutEmail = {
            ...mockPatient,
            telecom: [{system: 'phone', value: '+31612345678'}]
        };
        (useEnrollment as jest.Mock).mockReturnValue({
            patient: patientWithoutEmail,
            serviceRequest: mockServiceRequest,
            taskCondition: mockTaskCondition,
            isLoading: false
        });
        render(<EnrollmentDetails/>);
        const emailRow = screen.getByText('E-mailadres:').nextElementSibling;
        expect(emailRow).toHaveTextContent('Onbekend');
    });

    it('displays phone number when telecom phone is available', () => {
        render(<EnrollmentDetails/>);
        expect(screen.getByText('+31612345678')).toBeInTheDocument();
    });

    it('displays onbekend when patient has no phone telecom', () => {
        const patientWithoutPhone = {
            ...mockPatient,
            telecom: [{system: 'email', value: 'john.doe@example.com'}]
        };
        (useEnrollment as jest.Mock).mockReturnValue({
            patient: patientWithoutPhone,
            serviceRequest: mockServiceRequest,
            taskCondition: mockTaskCondition,
            isLoading: false
        });
        render(<EnrollmentDetails/>);
        const phoneRow = screen.getByText('Telefoonnummer:').nextElementSibling;
        expect(phoneRow).toHaveTextContent('Onbekend');
    });

    it('displays onbekend when patient has no telecom array', () => {
        const patientWithoutTelecom = {...mockPatient, telecom: undefined};
        (useEnrollment as jest.Mock).mockReturnValue({
            patient: patientWithoutTelecom,
            serviceRequest: mockServiceRequest,
            taskCondition: mockTaskCondition,
            isLoading: false
        });
        render(<EnrollmentDetails/>);
        const emailRow = screen.getByText('E-mailadres:').nextElementSibling;
        const phoneRow = screen.getByText('Telefoonnummer:').nextElementSibling;
        expect(emailRow).toHaveTextContent('Onbekend');
        expect(phoneRow).toHaveTextContent('Onbekend');
    });

    it('displays service request display with first letter uppercase', () => {
        render(<EnrollmentDetails/>);
        expect(screen.getByText('cardiologie consult')).toBeInTheDocument();
        expect(screen.getByText('cardiologie consult')).toHaveClass('first-letter:uppercase');
    });

    it('displays condition text when available', () => {
        render(<EnrollmentDetails/>);
        expect(screen.getByText('hartfalen')).toBeInTheDocument();
    });

    it('falls back to condition coding display when text is missing', () => {
        const conditionWithoutText = {
            ...mockTaskCondition,
            code: {coding: [{display: 'heart failure'}]}
        };
        (useEnrollment as jest.Mock).mockReturnValue({
            patient: mockPatient,
            serviceRequest: mockServiceRequest,
            taskCondition: conditionWithoutText,
            isLoading: false
        });
        render(<EnrollmentDetails/>);
        expect(screen.getByText('heart failure')).toBeInTheDocument();
    });

    it('displays onbekend when condition has no code', () => {
        const conditionWithoutCode = {...mockTaskCondition, code: undefined};
        (useEnrollment as jest.Mock).mockReturnValue({
            patient: mockPatient,
            serviceRequest: mockServiceRequest,
            taskCondition: conditionWithoutCode,
            isLoading: false
        });
        render(<EnrollmentDetails/>);
        const diagnosisRow = screen.getByText('Diagnose:').nextElementSibling;
        expect(diagnosisRow).toHaveTextContent('Onbekend');
    });

    it('displays organization name from fhirRender function', () => {
        render(<EnrollmentDetails/>);
        expect(screen.getByText('Test Hospital')).toBeInTheDocument();
        expect(fhirRender.organizationName).toHaveBeenCalledWith(mockServiceRequest.performer[0]);
    });

    it('applies correct grid layout classes', () => {
        render(<EnrollmentDetails/>);
        const container = screen.getByText('Patiënt:').parentElement;
        expect(container).toHaveClass('grid', 'grid-cols-[1fr_2fr]', 'gap-y-4', 'w-[568px]');
    });

    it('applies font-medium class to all labels', () => {
        render(<EnrollmentDetails/>);
        expect(screen.getByText('Patiënt:')).toHaveClass('font-medium');
        expect(screen.getByText('E-mailadres:')).toHaveClass('font-medium');
        expect(screen.getByText('Telefoonnummer:')).toHaveClass('font-medium');
        expect(screen.getByText('Thuismonitoring voor:')).toHaveClass('font-medium');
        expect(screen.getByText('Uitvoerende organisatie:')).toHaveClass('font-medium');
    })

    it('displays the correct instruction text with serviceRequest display and performer', () => {
        (useEnrollment as jest.Mock).mockReturnValue({
            patient: mockPatient,
            serviceRequest: mockServiceRequest,
            taskCondition: mockTaskCondition,
            isLoading: false
        });
        render(<EnrollmentDetails/>);
        // The organizationNameShort is likely used in the instruction text, so we need to match the expected output
        const expectedText = `Je gaat deze patient aanmelden voor cardiologie consult van Test Hospital.`;
        expect(screen.getByText(expectedText)).toBeInTheDocument();
    });

    it('displays the fallback instruction text when serviceRequest display or performer is missing', () => {
        (useEnrollment as jest.Mock).mockReturnValue({
            patient: mockPatient,
            serviceRequest: undefined,
            taskCondition: mockTaskCondition,
            isLoading: false
        });
        render(<EnrollmentDetails/>);
        expect(screen.getByText('Je gaat deze patient aanmelden.')).toBeInTheDocument();
    });
});