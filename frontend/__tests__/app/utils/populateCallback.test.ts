/**
 * @jest-environment node
 */

import { fetchResourceCallback } from '@/app/utils/populateCallback';
import type { RequestConfig } from '@/app/utils/populateCallback';

// Mock fetch globally
global.fetch = jest.fn();

describe('fetchResourceCallback', () => {
  const mockRequestConfig: RequestConfig = {
    clientEndpoint: 'https://api.example.com',
    authToken: 'test-token'
  };

  beforeEach(() => {
    jest.clearAllMocks();
    (fetch as jest.Mock).mockResolvedValue({
      ok: true,
      json: async () => ({ data: 'test' })
    });
  });

  it('makes fetch request with correct headers when no auth token provided', () => {
    const configWithoutAuth: RequestConfig = {
      clientEndpoint: 'https://api.example.com',
      authToken: null
    };

    fetchResourceCallback('Patient/123', configWithoutAuth);

    expect(fetch).toHaveBeenCalledWith('https://api.example.com/Patient/123', {
      headers: {
        Accept: 'application/json;charset=utf-8'
      }
    });
  });

  it('adds trailing slash to client endpoint when missing', () => {
    const configWithoutSlash: RequestConfig = {
      clientEndpoint: 'https://api.example.com',
      authToken: null
    };

    fetchResourceCallback('Patient/123', configWithoutSlash);

    expect(fetch).toHaveBeenCalledWith('https://api.example.com/Patient/123', {
      headers: {
        Accept: 'application/json;charset=utf-8'
      }
    });
  });

  it('does not add trailing slash when client endpoint already has one', () => {
    const configWithSlash: RequestConfig = {
      clientEndpoint: 'https://api.example.com/',
      authToken: null
    };

    fetchResourceCallback('Patient/123', configWithSlash);

    expect(fetch).toHaveBeenCalledWith('https://api.example.com/Patient/123', {
      headers: {
        Accept: 'application/json;charset=utf-8'
      }
    });
  });

  it('uses absolute URL when query is absolute HTTP URL', () => {
    fetchResourceCallback('https://external-api.com/Patient/123', mockRequestConfig);

    expect(fetch).toHaveBeenCalledWith('https://external-api.com/Patient/123', {
      headers: {
        Accept: 'application/json;charset=utf-8'
      }
    });
  });

  it('uses absolute URL when query is absolute HTTPS URL', () => {
    fetchResourceCallback('https://secure-api.com/Patient/123', mockRequestConfig);

    expect(fetch).toHaveBeenCalledWith('https://secure-api.com/Patient/123', {
      headers: {
        Accept: 'application/json;charset=utf-8'
      }
    });
  });

  it('uses absolute URL when query is absolute FTP URL', () => {
    fetchResourceCallback('ftp://ftp.example.com/data.json', mockRequestConfig);

    expect(fetch).toHaveBeenCalledWith('ftp://ftp.example.com/data.json', {
      headers: {
        Accept: 'application/json;charset=utf-8'
      }
    });
  });

  it('constructs relative URL when query is not absolute', () => {
    fetchResourceCallback('Patient/123', mockRequestConfig);

    expect(fetch).toHaveBeenCalledWith('https://api.example.com/Patient/123', {
      headers: {
        Accept: 'application/json;charset=utf-8'
      }
    });
  });

  it('handles query starting with slash correctly', () => {
    fetchResourceCallback('/Patient/123', mockRequestConfig);

    expect(fetch).toHaveBeenCalledWith('https://api.example.com//Patient/123', {
      headers: {
        Accept: 'application/json;charset=utf-8'
      }
    });
  });

  it('handles empty query string', () => {
    fetchResourceCallback('', mockRequestConfig);

    expect(fetch).toHaveBeenCalledWith('https://api.example.com/', {
      headers: {
        Accept: 'application/json;charset=utf-8'
      }
    });
  });

  it('handles complex query with parameters', () => {
    fetchResourceCallback('Patient?name=John&birthdate=1990-01-01', mockRequestConfig);

    expect(fetch).toHaveBeenCalledWith('https://api.example.com/Patient?name=John&birthdate=1990-01-01', {
      headers: {
        Accept: 'application/json;charset=utf-8'
      }
    });
  });

  it('returns fetch promise directly', () => {
    const mockPromise = Promise.resolve({ ok: true });
    (fetch as jest.Mock).mockReturnValue(mockPromise);

    const result = fetchResourceCallback('Patient/123', mockRequestConfig);

    expect(result).toBe(mockPromise);
  });

  it('handles URLs with special characters in path', () => {
    fetchResourceCallback('Patient/test-123_special', mockRequestConfig);

    expect(fetch).toHaveBeenCalledWith('https://api.example.com/Patient/test-123_special', {
      headers: {
        Accept: 'application/json;charset=utf-8'
      }
    });
  });

  it('handles malformed URLs that do not match absolute URL regex', () => {
    fetchResourceCallback('http:/malformed', mockRequestConfig);

    expect(fetch).toHaveBeenCalledWith('https://api.example.com/http:/malformed', {
      headers: {
        Accept: 'application/json;charset=utf-8'
      }
    });
  });
});
