import { render, screen } from '@testing-library/react';
import '@testing-library/jest-dom';
import Loading from '@/app/enrollment/loading';

describe('Loading', () => {
  it('renders a spinner with correct class and title', () => {
    render(<Loading />);
    const spinner = screen.getByTitle('loading-spinner');
    expect(spinner).toBeInTheDocument();
    expect(spinner).toHaveClass('fixed', 'inset-0', 'flex', 'items-center', 'justify-center', 'bg-white/80', 'z-50');
  });

  it('spinner has correct size and color classes', () => {
    render(<Loading />);
    const spinner = screen.getByTitle('loading-spinner');
    const svg = spinner.querySelector('svg');
    expect(svg).toHaveClass('h-12', 'w-12', 'text-primary');
  });

  it('renders only one spinner', () => {
    render(<Loading />);
    const spinners = screen.getAllByTitle('loading-spinner');
    expect(spinners.length).toBe(1);
  });
});
