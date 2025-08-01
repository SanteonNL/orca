/**
 * @jest-environment node
 */

jest.mock('@/lib/fhirUtils', () => ({
  createScpClient: jest.fn(),
}));

import { getLaunchableApps } from '@/app/applaunch';
import { createScpClient } from '@/lib/fhirUtils';
import type { Identifier, Bundle, Endpoint } from 'fhir/r4';

describe('getLaunchableApps', () => {
  const mockOrganization: Identifier = {
    system: 'https://example.com/organizations',
    value: 'org-123'
  };

  const mockSearchFn = jest.fn();



  beforeEach(() => {
    jest.clearAllMocks();
    process.env.PATIENT_VIEWER_URL = 'https://test-viewer.example.com';
    (createScpClient as jest.Mock).mockReturnValue({
      search: mockSearchFn
    });
  });

  it('returns test app when patient viewer URL is available', async () => {
    const result = await getLaunchableApps(mockOrganization);

    expect(result).toEqual([{
      Name: 'Test App',
      URL: 'https://test-viewer.example.com'
    }]);
    expect(createScpClient).not.toHaveBeenCalled();
  });

  it('searches for endpoints when no test app URL is available', async () => {
    delete process.env.PATIENT_VIEWER_URL;
    const mockBundle: Bundle<Endpoint> = {
      resourceType: 'Bundle',
      type: 'searchset',
      entry: []
    };
    mockSearchFn.mockResolvedValue(mockBundle);

    const result = await getLaunchableApps(mockOrganization);

    expect(createScpClient).toHaveBeenCalled();
    expect(mockSearchFn).toHaveBeenCalledWith({
      resourceType: 'Endpoint',
      headers: {
        'Cache-Control': 'no-cache',
        'X-Scp-Entity-Identifier': 'https://example.com/organizations|org-123'
      }
    });
    expect(result).toEqual([]);
  });

  it('returns active web-oauth2 endpoints with correct connection type', async () => {
    delete process.env.PATIENT_VIEWER_URL;
    const mockBundle: Bundle<Endpoint> = {
      resourceType: 'Bundle',
      type: 'searchset',
      entry: [
        {
          resource: {
            resourceType: 'Endpoint',
            status: 'active',
            name: 'Clinical App',
            address: 'https://clinical.example.com',
            connectionType: {
              system: 'http://santeonnl.github.io/shared-care-planning/endpoint-connection-type',
              code: 'web-oauth2'
            },
            payloadType: []
          }
        }
      ]
    };

    mockSearchFn.mockResolvedValue(mockBundle);

    const result = await getLaunchableApps(mockOrganization);

    expect(result).toEqual([{
      Name: 'Clinical App',
      URL: 'https://clinical.example.com'
    }]);
  });

  it('filters out inactive endpoints', async () => {
    delete process.env.PATIENT_VIEWER_URL;
    const mockBundle: Bundle<Endpoint> = {
      resourceType: 'Bundle',
      type: 'searchset',
      entry: [
        {
          resource: {
            resourceType: 'Endpoint',
            status: 'off',
            name: 'Inactive App',
            address: 'https://inactive.example.com',
            connectionType: {
              system: 'http://santeonnl.github.io/shared-care-planning/endpoint-connection-type',
              code: 'web-oauth2'
            },
            payloadType: []
          }
        }
      ]
    };
    mockSearchFn.mockResolvedValue(mockBundle);
    const result = await getLaunchableApps(mockOrganization);

    expect(result).toEqual([]);
  });

  it('filters out endpoints with incorrect connection type system', async () => {
    delete process.env.PATIENT_VIEWER_URL;
    const mockBundle: Bundle<Endpoint> = {
      resourceType: 'Bundle',
      type: 'searchset',
      entry: [
        {
          resource: {
            resourceType: 'Endpoint',
            status: 'active',
            name: 'Wrong System App',
            address: 'https://wrong-system.example.com',
            connectionType: {
              system: 'http://different-system.com',
              code: 'web-oauth2'
            },
            payloadType: []
          }
        }
      ]
    };
    mockSearchFn.mockResolvedValue(mockBundle);
    const result = await getLaunchableApps(mockOrganization);

    expect(result).toEqual([]);
  });

  it('filters out endpoints with incorrect connection type code', async () => {
    delete process.env.PATIENT_VIEWER_URL;
    const mockBundle: Bundle<Endpoint> = {
      resourceType: 'Bundle',
      type: 'searchset',
      entry: [
        {
          resource: {
            resourceType: 'Endpoint',
            status: 'active',
            name: 'Wrong Code App',
            address: 'https://wrong-code.example.com',
            connectionType: {
              system: 'http://santeonnl.github.io/shared-care-planning/endpoint-connection-type',
              code: 'hl7-fhir-rest'
            },
            payloadType: []
          }
        }
      ]
    };
    mockSearchFn.mockResolvedValue(mockBundle);
    const result = await getLaunchableApps(mockOrganization);

    expect(result).toEqual([]);
  });

  it('filters out endpoints without names', async () => {
    delete process.env.PATIENT_VIEWER_URL;
    const mockBundle: Bundle<Endpoint> = {
      resourceType: 'Bundle',
      type: 'searchset',
      entry: [
        {
          resource: {
            resourceType: 'Endpoint',
            status: 'active',
            address: 'https://no-name.example.com',
            connectionType: {
              system: 'http://santeonnl.github.io/shared-care-planning/endpoint-connection-type',
              code: 'web-oauth2'
            },
            payloadType: []
          }
        }
      ]
    };
    mockSearchFn.mockResolvedValue(mockBundle);
    const result = await getLaunchableApps(mockOrganization);

    expect(result).toEqual([]);
  });

  it('filters out entries without resources', async () => {
    delete process.env.PATIENT_VIEWER_URL;
    const mockBundle: Bundle<Endpoint> = {
      resourceType: 'Bundle',
      type: 'searchset',
      entry: [
        {},
        {
          resource: {
            resourceType: 'Endpoint',
            status: 'active',
            name: 'Valid App',
            address: 'https://valid.example.com',
            connectionType: {
              system: 'http://santeonnl.github.io/shared-care-planning/endpoint-connection-type',
              code: 'web-oauth2'
            },
            payloadType: []
          }
        }
      ]
    };
    mockSearchFn.mockResolvedValue(mockBundle);
    const result = await getLaunchableApps(mockOrganization);

    expect(result).toEqual([{
      Name: 'Valid App',
      URL: 'https://valid.example.com'
    }]);
  });

  it('returns multiple valid endpoints', async () => {
    delete process.env.PATIENT_VIEWER_URL;
    const mockBundle: Bundle<Endpoint> = {
      resourceType: 'Bundle',
      type: 'searchset',
      entry: [
        {
          resource: {
            resourceType: 'Endpoint',
            status: 'active',
            name: 'App One',
            address: 'https://app1.example.com',
            connectionType: {
              system: 'http://santeonnl.github.io/shared-care-planning/endpoint-connection-type',
              code: 'web-oauth2'
            },
            payloadType: []
          }
        },
        {
          resource: {
            resourceType: 'Endpoint',
            status: 'active',
            name: 'App Two',
            address: 'https://app2.example.com',
            connectionType: {
              system: 'http://santeonnl.github.io/shared-care-planning/endpoint-connection-type',
              code: 'web-oauth2'
            },
            payloadType: []
          }
        }
      ]
    };

    mockSearchFn.mockResolvedValue(mockBundle);
    const result = await getLaunchableApps(mockOrganization);

    expect(result).toEqual([
      {
        Name: 'App One',
        URL: 'https://app1.example.com'
      },
      {
        Name: 'App Two',
        URL: 'https://app2.example.com'
      }
    ]);
  });

  it('returns empty array when bundle has no entries', async () => {
    delete process.env.PATIENT_VIEWER_URL;
    const mockBundle: Bundle<Endpoint> = {
      resourceType: 'Bundle',
      type: 'searchset',
      entry: undefined
    };
    mockSearchFn.mockResolvedValue(mockBundle);
    const result = await getLaunchableApps(mockOrganization);

    expect(result).toEqual([]);
  });

  it('handles organization identifier with special characters', async () => {
    delete process.env.PATIENT_VIEWER_URL;
    const specialOrganization: Identifier = {
      system: 'https://example.com/orgs',
      value: 'org-with-special@chars#123'
    };
    const mockBundle: Bundle<Endpoint> = {
      resourceType: 'Bundle',
      type: 'searchset',
      entry: []
    };
    mockSearchFn.mockResolvedValue(mockBundle);

    await getLaunchableApps(specialOrganization);

    expect(mockSearchFn).toHaveBeenCalledWith({
      resourceType: 'Endpoint',
      headers: {
        'Cache-Control': 'no-cache',
        'X-Scp-Entity-Identifier': 'https://example.com/orgs|org-with-special@chars#123'
      }
    });
  });
});
