import { render, screen } from '@testing-library/react';
import '@testing-library/jest-dom';
import Home from '@/app/enrollment/page';

describe('Enrollment Overview Page', () => {
  it('renders the main element with correct classes', async () => {
    const { container } = render(await Home());
    const main = container.querySelector('main');
    expect(main).toHaveClass('flex', 'min-h-screen', 'flex-col', 'items-center', 'justify-between', 'py-5');
  });

  it('displays the enrollment overview text', async () => {
    render(await Home());
    expect(screen.getByText('Enrollment Overview')).toBeInTheDocument();
  });
});
