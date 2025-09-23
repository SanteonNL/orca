/**
 * @jest-environment node
 */

import {codingToMessage, MessageType} from '@/lib/fhirUtils';

describe('codingToMessage', () => {

    it('no email', () => {
        const codings = [{code: 'E0001'}]
        expect(codingToMessage(codings)).toStrictEqual([MessageType.NoEmail]);
    });
    it('no phone', () => {
        const codings = [{code: 'E0002'}]
        expect(codingToMessage(codings)).toStrictEqual([MessageType.NoPhone]);
    });
    it('invalid email', () => {
        const codings = [{code: 'E0003'}]
        expect(codingToMessage(codings)).toStrictEqual([MessageType.InvalidEmail]);
    })
    it('invalid phone', () => {
        const codings = [{code: 'E0004'}]
        expect(codingToMessage(codings)).toStrictEqual([MessageType.InvalidPhone]);
    });
    it('unknown code', () => {
        const codings = [{code: '1'}]
        expect(codingToMessage(codings)).toStrictEqual(['Er is een onbekende fout opgetreden. Probeer het later opnieuw of neem contact op met de systeembeheerder: functioneelbeheer@zorgbijjou.nl. Vermeld daarbij de volgende code: 1']);
    });

});
