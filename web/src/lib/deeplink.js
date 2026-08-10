/**
 * Deep links into the overlay.
 *
 * A host application that has just filed a ticket knows its ID, and the person
 * who filed it is the one least willing to retype it. `?ticket=FLG-7X3K9Q`
 * carries it over, so "look up the ticket I just filed" lands on the ticket
 * rather than on the report form with the ID sitting on a clipboard.
 *
 * It pairs with `?token=`, which `device.js` reads and strips: the token is what
 * authorises the lookup, this is what the lookup is for. Unlike the token, the
 * ticket ID is not a credential — knowing one buys a 403 without the token that
 * owns it — so it is deliberately left in the address bar, where a reload keeps
 * you on the same ticket.
 */

/**
 * Flagit's ticket ID shape, mirrored from `internal/db/ids.go`.
 *
 * Case-sensitive, and not anchored to one length: six characters are minted
 * today, four to twelve are documented, and IDs older than the narrowing of the
 * alphabet still carry lowercase. Re-casing here would turn those into a 404.
 */
export const TICKET_ID_PATTERN = /^FLG-[0-9A-Za-z]{4,12}$/;

/**
 * The ticket ID this page was opened for, or '' when there is none.
 *
 * Anything that is not a well-formed ID is treated as no ID at all: the value
 * arrives from a URL anyone can edit, and the only thing to do with a malformed
 * one is show the lookup field, which is what '' does.
 *
 * @param {{search?: string}} [options]
 * @returns {string}
 */
export function resolveTicketId({ search } = {}) {
  const params = new URLSearchParams(search ?? globalThis.location?.search ?? '');
  const raw = (params.get('ticket') ?? '').trim();
  return TICKET_ID_PATTERN.test(raw) ? raw : '';
}
