/**
 * Thin client over the Flagit HTTP API.
 *
 * Every call resolves to the `data` field of the server's success envelope, or
 * throws an ApiError carrying a translation key so the UI never has to render
 * a raw server string to a person.
 */

/** An API failure with a translation key the UI can render. */
export class ApiError extends Error {
  /**
   * @param {string} messageKey i18n key, e.g. `error.forbidden`
   * @param {number} [status] HTTP status, 0 when the request never landed
   * @param {string} [detail] the server's own message, for admins and logs
   */
  constructor(messageKey, status = 0, detail = '') {
    super(detail || messageKey);
    this.name = 'ApiError';
    this.messageKey = messageKey;
    this.status = status;
    this.detail = detail;
  }
}

/** Map an HTTP status onto the message a person should read. */
function keyForStatus(status) {
  if (status === 403) return 'error.forbidden';
  if (status === 404) return 'error.notFound';
  if (status === 401) return 'admin.keyInvalid';
  return 'error.generic';
}

/**
 * Perform a request and unwrap the response envelope.
 *
 * @param {string} path
 * @param {{method?: string, body?: unknown, headers?: Record<string,string>, fetchImpl?: typeof fetch}} [options]
 */
async function request(path, { method = 'GET', body, headers = {}, fetchImpl } = {}) {
  const doFetch = fetchImpl ?? globalThis.fetch;

  const init = { method, headers: { ...headers } };
  if (body !== undefined) {
    init.headers['Content-Type'] = 'application/json';
    init.body = JSON.stringify(body);
  }

  let response;
  try {
    response = await doFetch(path, init);
  } catch (cause) {
    // The request never reached the server: offline, DNS, CORS, TLS.
    throw new ApiError('error.network', 0, cause?.message ?? '');
  }

  const payload = await readJson(response);

  if (!response.ok) {
    throw new ApiError(
      keyForStatus(response.status),
      response.status,
      payload === NOT_JSON ? '' : (payload?.error ?? ''),
    );
  }

  /*
   * A success has to carry the envelope, and this is where that is enforced.
   *
   * Every endpoint answers `{"data": …}`; nothing but a wrong hop can produce a
   * 2xx that does not parse. Returning "no data" for one reads to every caller
   * as a server that had nothing to give — an empty ticket list, a ticket with
   * no messages — which is a claim about the database made on the strength of a
   * body we could not read. That is how a misrouted admin listener serving its
   * own HTML turned into "No tickets yet" on a dashboard with tickets behind
   * it, and stayed invisible.
   *
   * So it fails instead, loudly, with the status attached: whatever answered,
   * it was not this API.
   */
  if (payload === NOT_JSON && !isBodiless(response.status)) {
    throw new ApiError('error.malformedResponse', response.status, 'response body was not JSON');
  }
  return payload === NOT_JSON ? null : (payload?.data ?? null);
}

/**
 * Whether the status is one HTTP defines as carrying no body, and so the one
 * case where nothing to parse is the correct answer rather than a wrong hop.
 */
function isBodiless(status) {
  return status === 204 || status === 205;
}

/**
 * Returned by readJson for a body that is not JSON at all.
 *
 * A sentinel rather than null, because a literal `null` body is valid JSON and
 * must stay distinguishable from one that could not be parsed.
 */
const NOT_JSON = Symbol('not json');

async function readJson(response) {
  try {
    return await response.json();
  } catch {
    return NOT_JSON;
  }
}

/* ------------------------------------------------------------- public API -- */

/**
 * Client for the app-facing API, authorised by the reporter's device token.
 *
 * @param {{deviceToken: string, baseUrl?: string, fetchImpl?: typeof fetch}} config
 */
export function createPublicClient({ deviceToken, baseUrl = '', fetchImpl }) {
  const auth = () => ({ 'X-Device-Token': deviceToken });

  return {
    /** File a new ticket. Returns the created ticket, including its ID. */
    createTicket(ticket) {
      return request(`${baseUrl}/api/tickets`, {
        method: 'POST',
        body: { ...ticket, deviceToken },
        fetchImpl,
      });
    },

    /** Load a ticket and its conversation. */
    getTicket(id) {
      return request(`${baseUrl}/api/tickets/${encodeURIComponent(id)}`, {
        headers: auth(),
        fetchImpl,
      });
    },

    /** Add a reply to a ticket. */
    postMessage(id, text) {
      return request(`${baseUrl}/api/tickets/${encodeURIComponent(id)}/messages`, {
        method: 'POST',
        body: { body: text },
        headers: auth(),
        fetchImpl,
      });
    },
  };
}

/* ------------------------------------------------------------- admin API -- */

/**
 * Ask the admin listener whether the dashboard has to sign in with a key.
 *
 * Fails closed, deliberately. Only an explicit `adminKeyRequired: false` from
 * a listener that actually serves this route means "come straight in". A 404
 * (the public listener does not route it at all), a network failure, a body of
 * some other shape, a value that is not the boolean false — all of them read
 * as "a key is required". A probe going wrong must never be the thing that
 * takes the sign-in screen away.
 *
 * @param {{baseUrl?: string, fetchImpl?: typeof fetch}} [config]
 * @returns {Promise<{adminKeyRequired: boolean}>}
 */
export async function fetchAuthMode({ baseUrl = '', fetchImpl } = {}) {
  try {
    const mode = await request(`${baseUrl}/internal/auth`, { fetchImpl });
    return { adminKeyRequired: mode?.adminKeyRequired !== false };
  } catch {
    return { adminKeyRequired: true };
  }
}

/**
 * Client for the internal API, authorised by the admin key.
 *
 * @param {{adminKey: string, baseUrl?: string, fetchImpl?: typeof fetch}} config
 */
export function createAdminClient({ adminKey, baseUrl = '', fetchImpl }) {
  const auth = () => ({ 'X-Admin-Key': adminKey });
  const call = (path, options = {}) =>
    request(`${baseUrl}${path}`, { ...options, headers: auth(), fetchImpl });

  return {
    /**
     * List one page of tickets, optionally narrowed by app, status or type.
     *
     * Resolves to the array of tickets. Use listTicketPage when the paging
     * metadata matters.
     */
    async listTickets(filter = {}) {
      const page = await this.listTicketPage(filter);
      return page.tickets;
    },

    /**
     * List one page of tickets with its paging metadata:
     * `{ tickets, total, limit, offset, hasMore }`.
     */
    async listTicketPage(filter = {}) {
      const query = new URLSearchParams();
      if (filter.app) query.set('app', filter.app);
      if (filter.status) query.set('status', filter.status);
      if (filter.type) query.set('type', filter.type);
      if (filter.limit !== undefined) query.set('limit', String(filter.limit));
      if (filter.offset !== undefined) query.set('offset', String(filter.offset));
      const suffix = query.toString() ? `?${query}` : '';

      const page = await call(`/internal/tickets${suffix}`);
      // An older server, or a stubbed client, may hand back a bare array.
      if (Array.isArray(page)) {
        return { tickets: page, total: page.length, limit: page.length, offset: 0, hasMore: false };
      }
      return {
        tickets: page?.tickets ?? [],
        total: page?.total ?? 0,
        limit: page?.limit ?? 0,
        offset: page?.offset ?? 0,
        hasMore: page?.hasMore ?? false,
      };
    },

    /** Load one ticket with its conversation and commits. */
    getTicket(id) {
      return call(`/internal/tickets/${encodeURIComponent(id)}`);
    },

    /**
     * Change a ticket's status and/or reply to the reporter. Omit status to
     * leave the ticket where it is and only add a comment.
     *
     * force skips workflow validation, for an admin correcting a ticket that
     * is in the wrong state.
     */
    updateTicket(id, { status, shippedVersion, comment, force } = {}) {
      const body = { comment: comment ?? '' };
      if (status !== undefined) body.status = status;
      if (shippedVersion !== undefined) body.shippedVersion = shippedVersion;
      if (force !== undefined) body.force = force;
      return call(`/internal/tickets/${encodeURIComponent(id)}`, { method: 'PATCH', body });
    },

    /** Reply to the reporter. */
    postMessage(id, text) {
      return call(`/internal/tickets/${encodeURIComponent(id)}/messages`, {
        method: 'POST',
        body: { body: text },
      });
    },

    /**
     * Apply one status to many tickets, e.g. marking a release shipped.
     *
     * force defaults to true: a release sweep routinely moves tickets straight
     * from open to shipped, which the normal workflow does not allow, and an
     * admin selecting rows in the dashboard has already made that decision.
     */
    batchUpdate(ticketIds, status, shippedVersion = '', force = true) {
      return call('/internal/tickets/batch', {
        method: 'POST',
        body: { ticketIds, status, shippedVersion, force },
      });
    },

    /** Every app Flagit has seen a ticket from. */
    listApps() {
      return call('/internal/apps');
    },

    /** Turn automatic processing on or off for one app. */
    updateApp(name, autoProcessEnabled) {
      return call(`/internal/apps/${encodeURIComponent(name)}`, {
        method: 'PATCH',
        body: { autoProcessEnabled },
      });
    },

    /** Read the global configuration. */
    getSettings() {
      return call('/internal/settings');
    },

    /** Patch the global configuration; omitted fields are left alone. */
    updateSettings(patch) {
      return call('/internal/settings', { method: 'PATCH', body: patch });
    },

    /**
     * Which build is running: `{ commit, short, known }`. Admin-only — the
     * deployed revision is never served on the public API.
     */
    getVersion() {
      return call('/internal/version');
    },
  };
}
