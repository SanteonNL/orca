import { codingLabel } from '../../../app/utils/mapping';

describe('codingLabel', () => {
    const cases = [
        {
            input: { system: 'http://snomed.info/sct', code: '719858009', display: 'should not use display' },
            expected: 'thuismonitoring',
            description: 'returns mapped label for known system|code',
        },
        {
            input: { system: 'http://snomed.info/sct', code: '84114007', display: 'should not use display' },
            expected: 'hartfalen',
            description: 'returns mapped label for another known system|code',
        },
        {
            input: { system: 'http://snomed.info/sct', code: '13645005', display: 'should not use display' },
            expected: 'COPD',
            description: 'returns mapped label for COPD',
        },
        {
            input: { system: 'http://snomed.info/sct', code: '195967001', display: 'should not use display' },
            expected: 'astma',
            description: 'returns mapped label for astma',
        },
        {
            input: { system: 'http://snomed.info/sct', code: 'unknown', display: 'fallback display' },
            expected: 'fallback display',
            description: 'returns display when system|code not found',
        },
        {
            input: { system: 'http://other.system', code: '123', display: 'other display' },
            expected: 'other display',
            description: 'returns display for unknown system',
        },
        {
            input: { code: '719858009', display: 'missing system' },
            expected: 'missing system',
            description: 'returns display when system is missing',
        },
        {
            input: { system: 'http://snomed.info/sct', display: 'missing code' },
            expected: 'missing code',
            description: 'returns display when code is missing',
        },
        {
            input: { display: 'only display' },
            expected: 'only display',
            description: 'returns display when only display is present',
        },
        {
            input: {},
            expected: undefined,
            description: 'returns undefined when nothing is present',
        },
    ];

    test.each(cases)('$description', ({ input, expected }) => {
        expect(codingLabel(input)).toBe(expected);
    });
});

