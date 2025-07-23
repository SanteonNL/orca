import { render, screen } from '@testing-library/react';
import '@testing-library/jest-dom';
import ListTasks from '@/app/enrollment/list/page';

jest.mock('@/app/enrollment/list/components/table', () => {
  return function MockTaskOverviewTable() {
    return <div data-testid="task-overview-table">Task Overview Table</div>;
  };
});

describe("List page tests", () => {
    it('renders the task overview table component', () => {
        render(<ListTasks/>);
        expect(screen.getByTestId('task-overview-table')).toBeInTheDocument();
    });

    it('applies correct styling classes to the card content', () => {
        render(<ListTasks/>);
        const cardContent = screen.getByText('Task Overview Table').closest('[class*="space-y-6"]');
        expect(cardContent).toHaveClass('space-y-6', 'px-0');
    });

    it('renders the card with correct border and shadow styling', () => {
        render(<ListTasks />);
        const card = document.querySelector('.border-0.shadow-none.px-0');
        expect(card).toBeInTheDocument();
    });

    it('renders card content without padding', () => {
        render(<ListTasks />);
        const cardContent = document.querySelector('.space-y-6.px-0');
        expect(cardContent).toBeInTheDocument();
    });

    it('does not render card header', () => {
        render(<ListTasks />);
        expect(screen.queryByRole('banner')).not.toBeInTheDocument();
    });

    it('renders only task overview table as main content', () => {
        render(<ListTasks />);
        const card = screen.getByTestId('task-overview-table').parentElement;
        expect(card?.children).toHaveLength(1);
    });
})