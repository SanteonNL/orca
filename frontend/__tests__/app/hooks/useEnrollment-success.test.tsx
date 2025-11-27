import useEnrollment from '@/app/hooks/enrollment-hook';
import { Condition, Patient, Practitioner, PractitionerRole, ServiceRequest } from 'fhir/r4';
import { renderHook, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import type { QueryClientConfig } from '@tanstack/react-query';

function CreateWrapper(options?: QueryClientConfig) {
  const queryClient = new QueryClient(options);
  return function Wrapper({ children }: { children: React.ReactNode }) {
    return <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>;
  };
}

const mockPatient: Patient = { resourceType: 'Patient', id: 'patient-1' };
const mockPractitioner: Practitioner = { resourceType: 'Practitioner', id: 'practitioner-1' };
const mockPractitionerRole: PractitionerRole = { resourceType: 'PractitionerRole', id: 'role-1' };
const mockServiceRequest: Partial<ServiceRequest> = { resourceType: 'ServiceRequest', id: 'sr-1', reasonReference: [{ reference: 'Condition/cond-1' }] };
const mockCondition: Partial<Condition> = { resourceType: 'Condition', id: 'cond-1' };

const mockPatientFn = jest.fn();
const mockPractitionerFn = jest.fn();
const mockPractitionerRoleFn = jest.fn();
const mockServiceRequestFn = jest.fn();
const mockConditionFn = jest.fn();

const mockEhrClient = {
  read: jest.fn(({ resourceType, id }) => {
    switch (resourceType) {
      case 'Patient': return mockPatientFn(id);
      case 'Practitioner': return mockPractitionerFn(id);
      case 'PractitionerRole': return mockPractitionerRoleFn(id);
      case 'ServiceRequest': return mockServiceRequestFn(id);
      case 'Condition': return mockConditionFn(id);
      default: return Promise.resolve(undefined);
    }
  })
};


const mockLaunchContext = jest.fn()

jest.mock('@/app/hooks/context-hook', () => ({
  useLaunchContext: () => ({ launchContext: mockLaunchContext() }),
  useClients: () => ({ ehrClient: mockEhrClient })
}))

describe('useEnrollment', () => {
  beforeEach(() => {
    jest.clearAllMocks();
    mockLaunchContext.mockImplementation(() => ({
      patient: 'Patient/patient-1',
      practitioner: 'Practitioner/practitioner-1',
      practitionerRole: 'PractitionerRole/role-1',
      serviceRequest: 'ServiceRequest/sr-1',
    }));
    mockPatientFn.mockImplementation(() => Promise.resolve(mockPatient));
    mockPractitionerFn.mockImplementation(() => Promise.resolve(mockPractitioner));
    mockPractitionerRoleFn.mockImplementation(() => Promise.resolve(mockPractitionerRole));
    mockServiceRequestFn.mockImplementation(() => Promise.resolve(mockServiceRequest));
    mockConditionFn.mockImplementation(() => Promise.resolve(mockCondition));
  });

  it('returns all resources successfully', async () => {
    const wrapper = CreateWrapper();

    const { result } = renderHook(() => useEnrollment(), { wrapper });
    await waitFor(() => expect(result.current.isLoading).toEqual(false));

    expect(result.current.patient).toEqual(mockPatient);
    expect(result.current.practitioner).toEqual(mockPractitioner);
    expect(result.current.practitionerRole).toEqual(mockPractitionerRole);
    expect(result.current.serviceRequest).toEqual(mockServiceRequest);
    expect(result.current.taskCondition).toEqual(mockCondition);
    expect(result.current.isError).toBe(false);
    expect(result.current.error).toBeNull();
  });

  it('retries only the condition query when it fails', async () => {
    const wrapper = CreateWrapper({
      defaultOptions: {
        queries: {
          retry: 1,
          retryDelay: 0
        },
      },
    });

    let conditionCallCount = 0;
    mockConditionFn.mockImplementation(id => {
      conditionCallCount++;
      if (conditionCallCount < 2) {
        return Promise.reject(new Error('Condition fetch failed'));
      }
      return Promise.resolve(mockCondition);
    });

    const { result } = renderHook(() => useEnrollment(), { wrapper });
    await waitFor(() => expect(result.current.isLoading).toEqual(false));

    // The condition query should have been retried, others only called once
    expect(conditionCallCount).toBeGreaterThan(1);
    expect(mockPatientFn).toHaveBeenCalledTimes(1);
    expect(mockPractitionerFn).toHaveBeenCalledTimes(1);
    expect(mockPractitionerRoleFn).toHaveBeenCalledTimes(1);
    expect(mockServiceRequestFn).toHaveBeenCalledTimes(1);
    expect(result.current.taskCondition).toEqual(mockCondition);
  });

  it('returns combined error when all queries fail', async () => {
    const wrapper = CreateWrapper({
      defaultOptions: {
        queries: {
          retry: 0,
          retryDelay: 0
        },
      },
    });
    mockPatientFn.mockImplementation(() => Promise.reject(new Error('Patient error')));
    mockPractitionerFn.mockImplementation(() => Promise.reject(new Error('Practitioner error')));
    mockPractitionerRoleFn.mockImplementation(() => Promise.reject(new Error('PractitionerRole error')));
    mockServiceRequestFn.mockImplementation(() => Promise.reject(new Error('ServiceRequest error')));
    mockConditionFn.mockImplementation(() => Promise.reject(new Error('Condition error')));

    const { result } = renderHook(() => useEnrollment(), { wrapper });
    await waitFor(() => expect(result.current.isError).toEqual(true));

    expect(result.current.error?.message).toMatch(/Patient error|Practitioner error|PractitionerRole error|ServiceRequest error|Condition error/);
  });
});
