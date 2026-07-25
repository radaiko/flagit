import { describe, it, expect } from 'vitest';
import {
  t,
  translations,
  LANGUAGES,
  DEFAULT_LANGUAGE,
  normaliseLanguage,
  detectLanguage,
  rememberLanguage,
  otherLanguage,
} from '../src/lib/i18n.js';

describe('translation coverage', () => {
  it('has both supported languages', () => {
    expect(LANGUAGES).toEqual(['en', 'de']);
    expect(DEFAULT_LANGUAGE).toBe('en');
  });

  it('translates every English key into German', () => {
    const missing = Object.keys(translations.en).filter((key) => !(key in translations.de));

    expect(missing).toEqual([]);
  });

  it('has no German keys without an English original', () => {
    const extra = Object.keys(translations.de).filter((key) => !(key in translations.en));

    expect(extra).toEqual([]);
  });

  it('has no blank translations', () => {
    for (const lang of LANGUAGES) {
      for (const [key, value] of Object.entries(translations[lang])) {
        expect(value.trim(), `${lang}.${key} is blank`).not.toBe('');
      }
    }
  });

  it('actually translates — German is not a copy of English', () => {
    // A handful of strings legitimately match (the product name, version
    // numbers); most must differ, or the German table is a stub.
    const identical = Object.keys(translations.en).filter(
      (key) => translations.en[key] === translations.de[key],
    );

    expect(identical.length).toBeLessThan(Object.keys(translations.en).length * 0.2);
  });

  it('covers every status the backend can return', () => {
    for (const status of ['open', 'in-progress', 'resolved', 'shipped', 'closed']) {
      expect(translations.en[`status.${status}`]).toBeTruthy();
      expect(translations.de[`status.${status}`]).toBeTruthy();
    }
  });
});

describe('t', () => {
  it('returns the translation for the requested language', () => {
    expect(t('create.submit', 'en')).toBe('Send report');
    expect(t('create.submit', 'de')).toBe('Meldung senden');
  });

  it('defaults to English', () => {
    expect(t('create.submit')).toBe('Send report');
  });

  it('falls back to English for an unknown language', () => {
    expect(t('create.submit', 'fr')).toBe('Send report');
  });

  it('returns the key itself when there is no translation', () => {
    // Visible nonsense beats invisible blank space when a key is missing.
    expect(t('nope.missing', 'en')).toBe('nope.missing');
    expect(t('nope.missing', 'de')).toBe('nope.missing');
  });

  it('substitutes placeholders', () => {
    expect(t('mass.done', 'en', { n: 3 })).toBe('Updated 3 tickets');
    expect(t('mass.partial', 'de', { n: 2, failed: 1 })).toContain('2');
    expect(t('mass.partial', 'de', { n: 2, failed: 1 })).toContain('1');
  });

  it('leaves the string alone when no variables are given', () => {
    expect(t('mass.done', 'en')).toBe('Updated {n} tickets');
  });
});

describe('normaliseLanguage', () => {
  it.each([
    ['de', 'de'],
    ['de-AT', 'de'],
    ['DE', 'de'],
    ['  en-GB  ', 'en'],
    ['en', 'en'],
  ])('maps %s to %s', (input, expected) => {
    expect(normaliseLanguage(input)).toBe(expected);
  });

  it.each([['fr'], ['zh-Hans'], [''], [null], [undefined], [42]])(
    'rejects %s',
    (input) => {
      expect(normaliseLanguage(input)).toBeNull();
    },
  );
});

describe('detectLanguage', () => {
  it('prefers an explicit choice in the URL', () => {
    const lang = detectLanguage({
      search: '?lang=de',
      storage: fakeStorage({ 'flagit.lang': 'en' }),
      navigatorLanguages: ['en-GB'],
    });

    expect(lang).toBe('de');
  });

  it('falls back to a choice made earlier in the session', () => {
    const lang = detectLanguage({
      search: '',
      storage: fakeStorage({ 'flagit.lang': 'de' }),
      navigatorLanguages: ['en-GB'],
    });

    expect(lang).toBe('de');
  });

  it('falls back to the browser preference', () => {
    const lang = detectLanguage({
      search: '',
      storage: fakeStorage(),
      navigatorLanguages: ['fr-FR', 'de-AT', 'en'],
    });

    expect(lang).toBe('de', 'the first supported preference wins');
  });

  it('falls back to English when nothing matches', () => {
    const lang = detectLanguage({
      search: '',
      storage: fakeStorage(),
      navigatorLanguages: ['fr-FR', 'ja'],
    });

    expect(lang).toBe('en');
  });

  it('ignores an unsupported language in the URL', () => {
    const lang = detectLanguage({
      search: '?lang=fr',
      storage: fakeStorage({ 'flagit.lang': 'de' }),
      navigatorLanguages: [],
    });

    expect(lang).toBe('de');
  });

  it('works with no options at all', () => {
    expect(LANGUAGES).toContain(detectLanguage());
  });
});

describe('rememberLanguage', () => {
  it('stores the choice', () => {
    const storage = fakeStorage();

    rememberLanguage('de', storage);

    expect(storage.getItem('flagit.lang')).toBe('de');
  });

  it('survives storage that refuses to write', () => {
    const storage = {
      getItem: () => null,
      setItem: () => {
        throw new Error('quota exceeded');
      },
    };

    expect(() => rememberLanguage('de', storage)).not.toThrow();
  });

  it('defaults to session storage', () => {
    rememberLanguage('de');

    expect(sessionStorage.getItem('flagit.lang')).toBe('de');
  });
});

describe('otherLanguage', () => {
  it('flips between the two', () => {
    expect(otherLanguage('en')).toBe('de');
    expect(otherLanguage('de')).toBe('en');
  });
});

function fakeStorage(initial = {}) {
  const data = { ...initial };
  return {
    getItem: (key) => data[key] ?? null,
    setItem: (key, value) => {
      data[key] = value;
    },
  };
}
