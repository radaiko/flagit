import { describe, it, expect, vi, afterEach } from 'vitest';
import { resolveDeviceToken, generateToken } from '../src/lib/device.js';

const UUID_SHAPE = /^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/;

function fakeStorage(initial = {}) {
  const data = { ...initial };
  return {
    getItem: (key) => data[key] ?? null,
    setItem: (key, value) => {
      data[key] = value;
    },
    data,
  };
}

afterEach(() => {
  vi.unstubAllGlobals();
});

describe('resolveDeviceToken', () => {
  it('prefers a token handed over in the URL', () => {
    const storage = fakeStorage({ 'flagit.deviceToken': 'stored-token' });

    const token = resolveDeviceToken({ search: '?token=url-token', storage });

    expect(token).toBe('url-token');
    expect(storage.data['flagit.deviceToken']).toBe(
      'url-token',
      'the URL token becomes the session token',
    );
  });

  it('reuses the token already in this session', () => {
    const storage = fakeStorage({ 'flagit.deviceToken': 'stored-token' });

    expect(resolveDeviceToken({ search: '', storage })).toBe('stored-token');
  });

  it('mints and stores a token when there is none', () => {
    const storage = fakeStorage();

    const token = resolveDeviceToken({ search: '', storage });

    expect(token).toMatch(UUID_SHAPE);
    expect(storage.data['flagit.deviceToken']).toBe(token);
  });

  it('is stable across calls once a token exists', () => {
    const storage = fakeStorage();

    const first = resolveDeviceToken({ search: '', storage });
    const second = resolveDeviceToken({ search: '', storage });

    expect(second).toBe(first);
  });

  it('ignores a blank token in the URL and in storage', () => {
    const storage = fakeStorage({ 'flagit.deviceToken': '   ' });

    const token = resolveDeviceToken({ search: '?token=%20%20', storage });

    expect(token).toMatch(UUID_SHAPE);
  });

  it('still returns a token when storage throws', () => {
    const hostile = {
      getItem: () => {
        throw new Error('blocked');
      },
      setItem: () => {
        throw new Error('blocked');
      },
    };

    expect(resolveDeviceToken({ search: '', storage: hostile })).toMatch(UUID_SHAPE);
  });

  it('falls back to session storage', () => {
    const token = resolveDeviceToken({ search: '' });

    expect(sessionStorage.getItem('flagit.deviceToken')).toBe(token);
  });
});

describe('generateToken', () => {
  it('produces a well-formed UUIDv4', () => {
    expect(generateToken()).toMatch(UUID_SHAPE);
  });

  it('produces a different token each time', () => {
    const tokens = new Set(Array.from({ length: 50 }, () => generateToken()));

    expect(tokens.size).toBe(50);
  });

  it('falls back to random bytes without randomUUID', () => {
    // An overlay served over plain HTTP has no secure context, so
    // crypto.randomUUID is unavailable there.
    vi.stubGlobal('crypto', { getRandomValues: globalThis.crypto.getRandomValues.bind(globalThis.crypto) });

    expect(generateToken()).toMatch(UUID_SHAPE);
  });

  it('sets the version and variant bits on the fallback path', () => {
    // All-zero bytes prove the bit fiddling happens rather than the value
    // being passed through.
    vi.stubGlobal('crypto', {
      getRandomValues: (array) => array.fill(0),
    });

    expect(generateToken()).toBe('00000000-0000-4000-8000-000000000000');
  });

  it('fails loudly when there is no randomness at all', () => {
    vi.stubGlobal('crypto', undefined);

    expect(() => generateToken()).toThrow(/randomness/);
  });
});
