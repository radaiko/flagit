/** Presentation helpers shared by the overlay and the dashboard. */

const LOCALES = { en: 'en-GB', de: 'de-DE' };

/**
 * Format an RFC 3339 timestamp as a date a person can scan.
 *
 * @param {string} iso
 * @param {string} lang
 * @returns {string} empty string when there is nothing to show
 */
export function formatDate(iso, lang = 'en') {
  const date = toDate(iso);
  if (!date) return '';
  return new Intl.DateTimeFormat(LOCALES[lang] ?? LOCALES.en, {
    day: 'numeric',
    month: 'short',
    year: 'numeric',
  }).format(date);
}

/**
 * Format a timestamp with the time of day, for conversation entries where the
 * ordering within a day matters.
 *
 * @param {string} iso
 * @param {string} lang
 * @returns {string}
 */
export function formatDateTime(iso, lang = 'en') {
  const date = toDate(iso);
  if (!date) return '';
  return new Intl.DateTimeFormat(LOCALES[lang] ?? LOCALES.en, {
    day: 'numeric',
    month: 'short',
    year: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  }).format(date);
}

function toDate(iso) {
  if (!iso) return null;
  const date = new Date(iso);
  return Number.isNaN(date.getTime()) ? null : date;
}

/** The five statuses, in lifecycle order. Mirrors the Go model. */
export const STATUSES = ['open', 'in-progress', 'resolved', 'shipped', 'closed'];

/** The two ticket types. */
export const TYPES = ['bug', 'feature'];
