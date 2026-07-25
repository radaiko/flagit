import { describe, it, expect, vi } from 'vitest';
import { ApiError, createPublicClient, createAdminClient } from '../src/lib/api.js';
import { makeTicket } from './helpers.js';

/** A fetch stub that records its calls and returns a canned response. */
function stubFetch({ ok = true, status = 200, body = { data: null } } = {}) {
  return vi.fn().mockResolvedValue({
    ok,
    status,
    json: async () => body,
  });
}

function lastCall(fetchImpl) {
  const [url, init] = fetchImpl.mock.calls.at(-1);
  return { url, init };
}

describe('ApiError', () => {
  it('carries a translation key, status and the server detail', () => {
    const error = new ApiError('error.forbidden', 403, 'invalid device token');

    expect(error).toBeInstanceOf(Error);
    expect(error.name).toBe('ApiError');
    expect(error.messageKey).toBe('error.forbidden');
    expect(error.status).toBe(403);
    expect(error.detail).toBe('invalid device token');
    expect(error.message).toBe('invalid device token');
  });

  it('falls back to the key as its message', () => {
    expect(new ApiError('error.network').message).toBe('error.network');
  });
});

describe('public client', () => {
  it('sends the device token in the body when filing a ticket', async () => {
    const fetchImpl = stubFetch({ body: { data: makeTicket() } });
    const client = createPublicClient({ deviceToken: 'token-123', fetchImpl });

    const ticket = await client.createTicket({ type: 'bug', title: 'Crash' });

    expect(ticket.id).toBe('FLG-7X3K9Q');
    const { url, init } = lastCall(fetchImpl);
    expect(url).toBe('/api/tickets');
    expect(init.method).toBe('POST');
    expect(init.headers['Content-Type']).toBe('application/json');
    expect(JSON.parse(init.body)).toMatchObject({
      type: 'bug',
      title: 'Crash',
      deviceToken: 'token-123',
    });
  });

  it('sends the device token as a header when reading a ticket', async () => {
    const fetchImpl = stubFetch({ body: { data: makeTicket() } });
    const client = createPublicClient({ deviceToken: 'token-123', fetchImpl });

    await client.getTicket('FLG-7X3K9Q');

    const { url, init } = lastCall(fetchImpl);
    expect(url).toBe('/api/tickets/FLG-7X3K9Q');
    expect(init.headers['X-Device-Token']).toBe('token-123');
  });

  it('posts a message', async () => {
    const fetchImpl = stubFetch({ body: { data: { id: 1, body: 'Still broken' } } });
    const client = createPublicClient({ deviceToken: 'token-123', fetchImpl });

    await client.postMessage('FLG-7X3K9Q', 'Still broken');

    const { url, init } = lastCall(fetchImpl);
    expect(url).toBe('/api/tickets/FLG-7X3K9Q/messages');
    expect(init.method).toBe('POST');
    expect(JSON.parse(init.body)).toEqual({ body: 'Still broken' });
  });

  it('escapes the ticket ID into the path', async () => {
    const fetchImpl = stubFetch({ body: { data: null } });
    const client = createPublicClient({ deviceToken: 'token-123', fetchImpl });

    await client.getTicket('a/b?c');

    expect(lastCall(fetchImpl).url).toBe('/api/tickets/a%2Fb%3Fc');
  });

  it('honours a base URL', async () => {
    const fetchImpl = stubFetch({ body: { data: null } });
    const client = createPublicClient({
      deviceToken: 'token-123',
      baseUrl: 'https://flagit.example',
      fetchImpl,
    });

    await client.getTicket('FLG-7X3K9Q');

    expect(lastCall(fetchImpl).url).toBe('https://flagit.example/api/tickets/FLG-7X3K9Q');
  });
});

describe('error mapping', () => {
  it.each([
    [403, 'error.forbidden'],
    [404, 'error.notFound'],
    [401, 'admin.keyInvalid'],
    [400, 'error.generic'],
    [500, 'error.generic'],
  ])('maps HTTP %i to %s', async (status, messageKey) => {
    const fetchImpl = stubFetch({ ok: false, status, body: { error: 'server said no' } });
    const client = createPublicClient({ deviceToken: 'token', fetchImpl });

    await expect(client.getTicket('FLG-7X3K9Q')).rejects.toMatchObject({
      messageKey,
      status,
      detail: 'server said no',
    });
  });

  it('reports a failed request as a network error', async () => {
    const fetchImpl = vi.fn().mockRejectedValue(new TypeError('Failed to fetch'));
    const client = createPublicClient({ deviceToken: 'token', fetchImpl });

    await expect(client.getTicket('FLG-7X3K9Q')).rejects.toMatchObject({
      messageKey: 'error.network',
      status: 0,
      detail: 'Failed to fetch',
    });
  });

  it('survives a rejection with no message', async () => {
    const fetchImpl = vi.fn().mockRejectedValue(undefined);
    const client = createPublicClient({ deviceToken: 'token', fetchImpl });

    await expect(client.getTicket('FLG-7X3K9Q')).rejects.toMatchObject({
      messageKey: 'error.network',
      detail: '',
    });
  });

  it('survives an error response that is not JSON', async () => {
    const fetchImpl = vi.fn().mockResolvedValue({
      ok: false,
      status: 502,
      json: async () => {
        throw new SyntaxError('Unexpected token <');
      },
    });
    const client = createPublicClient({ deviceToken: 'token', fetchImpl });

    await expect(client.getTicket('FLG-7X3K9Q')).rejects.toMatchObject({
      messageKey: 'error.generic',
      detail: '',
    });
  });

  it('returns null for a success response with no body', async () => {
    const fetchImpl = vi.fn().mockResolvedValue({
      ok: true,
      status: 204,
      json: async () => {
        throw new SyntaxError('no body');
      },
    });
    const client = createPublicClient({ deviceToken: 'token', fetchImpl });

    await expect(client.getTicket('FLG-7X3K9Q')).resolves.toBeNull();
  });
});

describe('admin client', () => {
  const build = (options) => {
    const fetchImpl = stubFetch(options);
    return { fetchImpl, client: createAdminClient({ adminKey: 'admin-key', fetchImpl }) };
  };

  it('sends the admin key on every call', async () => {
    const { fetchImpl, client } = build({ body: { data: [] } });

    await client.listTickets();

    expect(lastCall(fetchImpl).init.headers['X-Admin-Key']).toBe('admin-key');
  });

  it('builds a filter query from the filters that are set', async () => {
    const { fetchImpl, client } = build({ body: { data: [] } });

    await client.listTickets({ app: 'notes', status: 'open', type: 'bug' });

    expect(lastCall(fetchImpl).url).toBe('/internal/tickets?app=notes&status=open&type=bug');
  });

  it('omits empty filters', async () => {
    const { fetchImpl, client } = build({ body: { data: [] } });

    await client.listTickets({ app: '', status: 'open' });

    expect(lastCall(fetchImpl).url).toBe('/internal/tickets?status=open');
  });

  it('sends no query string when nothing is filtered', async () => {
    const { fetchImpl, client } = build({ body: { data: [] } });

    await client.listTickets();

    expect(lastCall(fetchImpl).url).toBe('/internal/tickets');
  });

  it('loads one ticket', async () => {
    const { fetchImpl, client } = build({ body: { data: makeTicket() } });

    const ticket = await client.getTicket('FLG-7X3K9Q');

    expect(ticket.id).toBe('FLG-7X3K9Q');
    expect(lastCall(fetchImpl).url).toBe('/internal/tickets/FLG-7X3K9Q');
  });

  it('patches a ticket, defaulting the optional fields', async () => {
    const { fetchImpl, client } = build({ body: { data: makeTicket() } });

    await client.updateTicket('FLG-7X3K9Q', { status: 'resolved' });

    const { url, init } = lastCall(fetchImpl);
    expect(url).toBe('/internal/tickets/FLG-7X3K9Q');
    expect(init.method).toBe('PATCH');
    expect(JSON.parse(init.body)).toEqual({
      status: 'resolved',
      shippedVersion: '',
      comment: '',
    });
  });

  it('patches a ticket with a version and a comment', async () => {
    const { fetchImpl, client } = build({ body: { data: makeTicket() } });

    await client.updateTicket('FLG-7X3K9Q', {
      status: 'shipped',
      shippedVersion: '1.5.0',
      comment: 'Out now',
    });

    expect(JSON.parse(lastCall(fetchImpl).init.body)).toEqual({
      status: 'shipped',
      shippedVersion: '1.5.0',
      comment: 'Out now',
    });
  });

  it('patches a ticket called with no options', async () => {
    const { fetchImpl, client } = build({ body: { data: makeTicket() } });

    await client.updateTicket('FLG-7X3K9Q');

    expect(JSON.parse(lastCall(fetchImpl).init.body)).toEqual({
      shippedVersion: '',
      comment: '',
    });
  });

  it('posts an agent message', async () => {
    const { fetchImpl, client } = build({ body: { data: {} } });

    await client.postMessage('FLG-7X3K9Q', 'On it');

    const { url, init } = lastCall(fetchImpl);
    expect(url).toBe('/internal/tickets/FLG-7X3K9Q/messages');
    expect(JSON.parse(init.body)).toEqual({ body: 'On it' });
  });

  it('applies a batch update', async () => {
    const { fetchImpl, client } = build({ body: { data: { updated: [], failed: {} } } });

    await client.batchUpdate(['FLG-A', 'FLG-B'], 'shipped', '1.5.0');

    const { url, init } = lastCall(fetchImpl);
    expect(url).toBe('/internal/tickets/batch');
    expect(JSON.parse(init.body)).toEqual({
      ticketIds: ['FLG-A', 'FLG-B'],
      status: 'shipped',
      shippedVersion: '1.5.0',
    });
  });

  it('defaults the batch version to empty', async () => {
    const { fetchImpl, client } = build({ body: { data: {} } });

    await client.batchUpdate(['FLG-A'], 'closed');

    expect(JSON.parse(lastCall(fetchImpl).init.body).shippedVersion).toBe('');
  });

  it('lists apps', async () => {
    const { fetchImpl, client } = build({ body: { data: [] } });

    await client.listApps();

    expect(lastCall(fetchImpl).url).toBe('/internal/apps');
  });

  it('patches an app', async () => {
    const { fetchImpl, client } = build({ body: { data: {} } });

    await client.updateApp('my app', true);

    const { url, init } = lastCall(fetchImpl);
    expect(url).toBe('/internal/apps/my%20app');
    expect(init.method).toBe('PATCH');
    expect(JSON.parse(init.body)).toEqual({ autoProcessEnabled: true });
  });

  it('reads and patches settings', async () => {
    const { fetchImpl, client } = build({ body: { data: { globalAutoProcess: true } } });

    await client.getSettings();
    expect(lastCall(fetchImpl).url).toBe('/internal/settings');

    await client.updateSettings({ globalAutoProcess: true });
    const { init } = lastCall(fetchImpl);
    expect(init.method).toBe('PATCH');
    expect(JSON.parse(init.body)).toEqual({ globalAutoProcess: true });
  });

  it('surfaces a rejected admin key as a key error', async () => {
    const { client } = build({ ok: false, status: 401, body: { error: 'invalid admin key' } });

    await expect(client.getSettings()).rejects.toMatchObject({ messageKey: 'admin.keyInvalid' });
  });
});
