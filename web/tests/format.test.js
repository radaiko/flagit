import { describe, it, expect } from 'vitest';
import { formatDate, formatDateTime, STATUSES, TYPES } from '../src/lib/format.js';

describe('formatDate', () => {
  it('formats in English', () => {
    expect(formatDate('2026-07-20T09:00:00Z', 'en')).toBe('20 Jul 2026');
  });

  it('formats in German', () => {
    // German uses a different month abbreviation and order.
    expect(formatDate('2026-07-20T09:00:00Z', 'de')).toMatch(/2026/);
    expect(formatDate('2026-07-20T09:00:00Z', 'de')).not.toBe(
      formatDate('2026-07-20T09:00:00Z', 'en'),
    );
  });

  it('defaults to English for an unknown language', () => {
    expect(formatDate('2026-07-20T09:00:00Z', 'fr')).toBe(
      formatDate('2026-07-20T09:00:00Z', 'en'),
    );
  });

  it('defaults the language argument', () => {
    expect(formatDate('2026-07-20T09:00:00Z')).toBe('20 Jul 2026');
  });

  it.each([[''], [null], [undefined], ['not-a-date']])(
    'returns an empty string for %s',
    (input) => {
      expect(formatDate(input, 'en')).toBe('');
    },
  );
});

describe('formatDateTime', () => {
  it('includes the time of day', () => {
    const formatted = formatDateTime('2026-07-20T09:05:00Z', 'en');

    expect(formatted).toContain('2026');
    expect(formatted).toMatch(/\d{2}:\d{2}/);
  });

  it('returns an empty string for junk', () => {
    expect(formatDateTime('nonsense', 'en')).toBe('');
    expect(formatDateTime('', 'de')).toBe('');
  });

  it('defaults the language argument', () => {
    expect(formatDateTime('2026-07-20T09:05:00Z')).toMatch(/2026/);
  });

  it('falls back to English for an unknown language', () => {
    expect(formatDateTime('2026-07-20T09:05:00Z', 'fr')).toBe(
      formatDateTime('2026-07-20T09:05:00Z', 'en'),
    );
  });
});

describe('constants', () => {
  it('lists the statuses in lifecycle order', () => {
    expect(STATUSES).toEqual(['open', 'in-progress', 'resolved', 'shipped', 'closed']);
  });

  it('lists both ticket types', () => {
    expect(TYPES).toEqual(['bug', 'feature']);
  });
});
