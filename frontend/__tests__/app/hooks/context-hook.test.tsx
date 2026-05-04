import { renderHook, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { useLaunchContext, useClients } from '@/app/hooks/context-hook';

// Mock fetch globally
const mockFetch = jest.fn();
globalThis.fetch = mockFetch;

function CreateWrapper() {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: {
        retry: false,
      },
    },
  });
  return function Wrapper({ children }: { children: React.ReactNode }) {
    return <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>;
  };
}

describe('useLaunchContext', () => {
  beforeEach(() => {
    jest.clearAllMocks();
  });

  it('fetches and returns launch context successfully', async () => {
    const mockLaunchContext = {
      patient: 'Patient/123',
      practitioner: 'Practitioner/456',
      practitionerRole: 'PractitionerRole/789',
      tenantId: 'tenant-123',
      serviceRequest: 'ServiceRequest/abc',
    };

    mockFetch.mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => mockLaunchContext,
    });

    const wrapper = CreateWrapper();
    const { result } = renderHook(() => useLaunchContext(), { wrapper });

    await waitFor(() => expect(result.current.isLoading).toBe(false));

    expect(result.current.launchContext).toEqual(mockLaunchContext);
    expect(result.current.isError).toBe(false);
    expect(result.current.error).toBeNull();
    expect(mockFetch).toHaveBeenCalledWith('/orca/cpc/context');
  });

  it('uses staleTime for caching', async () => {
    const mockLaunchContext = {
      patient: 'Patient/123',
      practitioner: 'Practitioner/456',
      practitionerRole: 'PractitionerRole/789',
      tenantId: 'tenant-123',
    };

    mockFetch.mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => mockLaunchContext,
    });

    const wrapper = CreateWrapper();
    const { result, rerender } = renderHook(() => useLaunchContext(), { wrapper });

    await waitFor(() => expect(result.current.isLoading).toBe(false));

    // Clear the mock to see if it gets called again
    mockFetch.mockClear();

    // Rerender should use cached data
    rerender();

    expect(mockFetch).not.toHaveBeenCalled();
    expect(result.current.launchContext).toEqual(mockLaunchContext);
  });
});

describe('useClients', () => {
  beforeEach(() => {
    jest.clearAllMocks();
    // Mock window.location for client creation
    globalThis.window = { location: { origin: 'http://localhost:3000' } } as any;
  });

  afterEach(() => {
    delete (globalThis as any).window;
  });

  it('returns undefined clients when no launch context', async () => {
    mockFetch.mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => ({
        patient: 'Patient/123',
        practitioner: 'Practitioner/456',
        practitionerRole: 'PractitionerRole/789',
        // No tenantId
      }),
    });

    const wrapper = CreateWrapper();
    const { result } = renderHook(() => {
      useLaunchContext(); // Trigger launch context loading
      return useClients();
    }, { wrapper });

    await waitFor(() => {
      expect(result.current.ehrClient).toBeUndefined();
      expect(result.current.cpsClient).toBeUndefined();
      expect(result.current.scpClient).toBeUndefined();
    });
  });

  it('creates FHIR clients when tenantId is available', async () => {
    const mockLaunchContext = {
      patient: 'Patient/123',
      practitioner: 'Practitioner/456',
      practitionerRole: 'PractitionerRole/789',
      tenantId: 'tenant-123',
    };

    mockFetch.mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => mockLaunchContext,
    });

    const wrapper = CreateWrapper();
    const { result: contextResult } = renderHook(() => useLaunchContext(), { wrapper });

    await waitFor(() => expect(contextResult.current.isLoading).toBe(false));

    const { result: clientsResult } = renderHook(() => useClients(), { wrapper });

    expect(clientsResult.current.ehrClient).toBeDefined();
    expect(clientsResult.current.cpsClient).toBeDefined();
    expect(clientsResult.current.scpClient).toBeDefined();
  });

  it('memoizes clients based on tenantId', async () => {
    const mockLaunchContext = {
      patient: 'Patient/123',
      practitioner: 'Practitioner/456',
      practitionerRole: 'PractitionerRole/789',
      tenantId: 'tenant-123',
    };

    mockFetch.mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => mockLaunchContext,
    });

    const wrapper = CreateWrapper();
    
    // First render
    const { result: contextResult } = renderHook(() => useLaunchContext(), { wrapper });
    await waitFor(() => expect(contextResult.current.isLoading).toBe(false));

    const { result: clientsResult, rerender } = renderHook(() => useClients(), { wrapper });
    
    const firstClients = {
      ehrClient: clientsResult.current.ehrClient,
      cpsClient: clientsResult.current.cpsClient,
      scpClient: clientsResult.current.scpClient,
    };

    // Rerender
    rerender();

    // Should return same instances (memoized)
    expect(clientsResult.current.ehrClient).toBe(firstClients.ehrClient);
    expect(clientsResult.current.cpsClient).toBe(firstClients.cpsClient);
    expect(clientsResult.current.scpClient).toBe(firstClients.scpClient);
  });
});
