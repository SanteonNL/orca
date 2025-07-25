import { render } from '@testing-library/react';
import TaskSseConnectionStatus from '@/app/enrollment/components/sse-connection-status';
import '@testing-library/jest-dom';

let connected = true;

jest.mock('@/lib/store/task-progress-store', () => ({
    __esModule: true,
    default: jest.fn(() => ({
        eventSourceConnected: connected,
    })),
}));

beforeEach(() => {
    jest.clearAllMocks();
    connected = true;
});

describe('TaskSseConnectionStatus', () => {
  it('shows green indicator when connected', () => {
    render(<TaskSseConnectionStatus />);
    const indicator = document.querySelector('.bg-green-500');
    expect(indicator).toBeInTheDocument();
  });


  it('shows red indicator when disconnected', () => {
    connected = false;
    render(<TaskSseConnectionStatus />);
    const indicator = document.querySelector('.bg-red-500');
    expect(indicator).toBeInTheDocument();
  });

});
