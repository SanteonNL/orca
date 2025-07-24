/**
 * @jest-environment node
 */

import { getSupportContactLink, getPatientViewerTestUrl } from '@/app/actions';

describe('getSupportContactLink', () => {
  const originalEnv = process.env;

  beforeEach(() => {
    jest.resetModules();
    process.env = { ...originalEnv };
  });

  afterAll(() => {
    process.env = originalEnv;
  });

  it('returns support contact link when environment variable is set', async () => {
    process.env.SUPPORT_CONTACT_LINK = 'https://support.example.com';

    const result = await getSupportContactLink();

    expect(result).toBe('https://support.example.com');
  });

  it('returns undefined when environment variable is not set', async () => {
    delete process.env.SUPPORT_CONTACT_LINK;

    const result = await getSupportContactLink();

    expect(result).toBeUndefined();
  });

  it('returns empty string when environment variable is empty', async () => {
    process.env.SUPPORT_CONTACT_LINK = '';

    const result = await getSupportContactLink();

    expect(result).toBe('');
  });
});

describe('getPatientViewerTestUrl', () => {
  const originalEnv = process.env;

  beforeEach(() => {
    jest.resetModules();
    process.env = { ...originalEnv };
  });

  afterAll(() => {
    process.env = originalEnv;
  });

  it('returns patient viewer URL when environment variable is set', async () => {
    process.env.PATIENT_VIEWER_URL = 'https://viewer.example.com';

    const result = await getPatientViewerTestUrl();

    expect(result).toBe('https://viewer.example.com');
  });

  it('returns undefined when environment variable is not set', async () => {
    delete process.env.PATIENT_VIEWER_URL;

    const result = await getPatientViewerTestUrl();

    expect(result).toBeUndefined();
  });

  it('returns empty string when environment variable is empty', async () => {
    process.env.PATIENT_VIEWER_URL = '';

    const result = await getPatientViewerTestUrl();

    expect(result).toBe('');
  });

  it('handles URL with query parameters', async () => {
    process.env.PATIENT_VIEWER_URL = 'https://viewer.example.com?param=value';

    const result = await getPatientViewerTestUrl();

    expect(result).toBe('https://viewer.example.com?param=value');
  });

  it('handles localhost URLs', async () => {
    process.env.PATIENT_VIEWER_URL = 'http://localhost:3000';

    const result = await getPatientViewerTestUrl();

    expect(result).toBe('http://localhost:3000');
  });
});
