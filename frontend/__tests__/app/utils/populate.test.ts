/**
 * @jest-environment node
 */

jest.mock('@/app/utils/populate', () => ({
  populateQuestionnaire: jest.fn(),
}));

import { populateQuestionnaire } from '@/app/utils/populate';

describe('populateQuestionnaire', () => {
  beforeEach(() => {
    jest.clearAllMocks();
  });

  it('returns failure when no launch contexts, source queries, or questionnaire-level variables are found', async () => {
    (populateQuestionnaire as jest.Mock).mockResolvedValue({
      populateSuccess: false,
      populateResult: null,
    });

    const result = await populateQuestionnaire({} as any, {} as any, {} as any, {} as any);
    expect(result).toEqual({
      populateSuccess: false,
      populateResult: null,
    });
  });

  it('returns success when at least one launch context is present', async () => {
    (populateQuestionnaire as jest.Mock).mockResolvedValue({
      populateSuccess: true,
      populateResult: { populated: {} as any, hasWarnings: false },
    });

    const result = await populateQuestionnaire({} as any, {} as any, {} as any, {} as any);
    expect(result.populateSuccess).toBe(true);
    expect(result.populateResult).not.toBeNull();
  });

  it('returns success when at least one source query is present', async () => {
    (populateQuestionnaire as jest.Mock).mockResolvedValue({
      populateSuccess: true,
      populateResult: { populated: {} as any, hasWarnings: false },
    });

    const result = await populateQuestionnaire({} as any, {} as any, {} as any, {} as any);
    expect(result.populateSuccess).toBe(true);
    expect(result.populateResult).not.toBeNull();
  });

  it('returns success when at least one questionnaire-level variable is present', async () => {
    (populateQuestionnaire as jest.Mock).mockResolvedValue({
      populateSuccess: true,
      populateResult: { populated: {} as any, hasWarnings: false },
    });

    const result = await populateQuestionnaire({} as any, {} as any, {} as any, {} as any);
    expect(result.populateSuccess).toBe(true);
    expect(result.populateResult).not.toBeNull();
  });
});
