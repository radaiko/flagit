import { vi } from 'vitest';

/** A ticket shaped like the API's response, overridable per test. */
export function makeTicket(overrides = {}) {
  return {
    id: 'FLG-7X3K9Q',
    type: 'bug',
    title: 'Crash on save',
    body: 'Tapping save closes the app',
    status: 'open',
    appName: 'notes',
    appVersion: '1.4.2',
    os: 'iOS 18.2',
    platform: 'ios',
    deviceModel: 'iPhone 15',
    shippedVersion: '',
    createdAt: '2026-07-20T09:00:00Z',
    updatedAt: '2026-07-21T11:30:00Z',
    messages: [],
    commits: [],
    ...overrides,
  };
}

export function makeMessage(overrides = {}) {
  return {
    id: 1,
    ticketId: 'FLG-7X3K9Q',
    body: 'Looking into it',
    role: 'agent',
    createdAt: '2026-07-21T11:30:00Z',
    ...overrides,
  };
}

export function makeApp(overrides = {}) {
  return {
    name: 'notes',
    autoProcessEnabled: false,
    createdAt: '2026-07-01T08:00:00Z',
    ...overrides,
  };
}

/**
 * A stub public client. Every method is a vi.fn() so tests can assert calls and
 * override resolutions without touching the network.
 */
export function stubPublicClient(overrides = {}) {
  return {
    createTicket: vi.fn().mockResolvedValue(makeTicket()),
    getTicket: vi.fn().mockResolvedValue(makeTicket()),
    postMessage: vi.fn().mockResolvedValue(makeMessage({ role: 'user', body: 'Still broken' })),
    ...overrides,
  };
}

/** A stub admin client covering the whole internal API surface. */
export function stubAdminClient(overrides = {}) {
  return {
    listTickets: vi.fn().mockResolvedValue([makeTicket()]),
    getTicket: vi.fn().mockResolvedValue(makeTicket()),
    updateTicket: vi.fn().mockResolvedValue(makeTicket({ status: 'resolved' })),
    postMessage: vi.fn().mockResolvedValue(makeMessage()),
    batchUpdate: vi.fn().mockResolvedValue({ updated: ['FLG-7X3K9Q'], failed: {} }),
    deleteTicket: vi.fn().mockResolvedValue({ id: 'FLG-7X3K9Q', deleted: true }),
    deleteTickets: vi.fn().mockResolvedValue({ deleted: ['FLG-7X3K9Q'], missing: [] }),
    listApps: vi.fn().mockResolvedValue([makeApp()]),
    updateApp: vi.fn().mockResolvedValue(makeApp({ autoProcessEnabled: true })),
    getSettings: vi.fn().mockResolvedValue({ globalAutoProcess: false, hermesWebhookUrl: '' }),
    getVersion: vi.fn().mockResolvedValue({
      commit: '212b000f1e2d3c4b5a69788796a5b4c3d2e1f0aa',
      short: '212b000',
      known: true,
    }),
    updateSettings: vi
      .fn()
      .mockResolvedValue({ globalAutoProcess: true, hermesWebhookUrl: 'https://hermes.example/hook' }),
    ...overrides,
  };
}

/** Build a rejected-promise error shaped like ApiError, without importing it. */
export function apiError(messageKey, status = 400) {
  const error = new Error(messageKey);
  error.messageKey = messageKey;
  error.status = status;
  return error;
}
