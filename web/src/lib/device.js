/**
 * The device ownership token.
 *
 * This token is the only credential in Flagit: whoever holds it owns the
 * tickets filed with it. The server stores a SHA-256 hash, never the token, so
 * losing it means losing access to those tickets — there is no recovery and no
 * account to fall back on. That is the trade for asking nobody to sign up.
 */

const STORAGE_KEY = 'flagit.deviceToken';

/**
 * Resolve the device token, in order of authority: the host app passed one in
 * the URL, this session already has one, or we mint one now.
 *
 * A token from the URL is written to session storage so the rest of the
 * session agrees with it.
 *
 * @param {{search?: string, storage?: Storage}} [options]
 * @returns {string}
 */
export function resolveDeviceToken({ search, storage } = {}) {
  const store = storage ?? safeSessionStorage();
  const params = new URLSearchParams(search ?? globalThis.location?.search ?? '');

  const fromUrl = (params.get('token') ?? '').trim();
  if (fromUrl) {
    write(store, fromUrl);
    return fromUrl;
  }

  const existing = (read(store) ?? '').trim();
  if (existing) return existing;

  const generated = generateToken();
  write(store, generated);
  return generated;
}

/**
 * Mint a random token. Prefers the platform UUID generator and falls back to
 * random bytes, because `crypto.randomUUID` needs a secure context and an
 * overlay may be loaded over plain HTTP on a local network.
 *
 * @returns {string}
 */
export function generateToken() {
  const crypto = globalThis.crypto;
  if (typeof crypto?.randomUUID === 'function') {
    return crypto.randomUUID();
  }
  if (typeof crypto?.getRandomValues === 'function') {
    const bytes = crypto.getRandomValues(new Uint8Array(16));
    // Set the version and variant bits so the result is a well-formed UUIDv4.
    bytes[6] = (bytes[6] & 0x0f) | 0x40;
    bytes[8] = (bytes[8] & 0x3f) | 0x80;
    const hex = [...bytes].map((b) => b.toString(16).padStart(2, '0')).join('');
    return `${hex.slice(0, 8)}-${hex.slice(8, 12)}-${hex.slice(12, 16)}-${hex.slice(16, 20)}-${hex.slice(20)}`;
  }
  throw new Error('no source of randomness available for a device token');
}

function read(store) {
  try {
    return store?.getItem(STORAGE_KEY) ?? null;
  } catch {
    return null;
  }
}

function write(store, value) {
  try {
    store?.setItem(STORAGE_KEY, value);
  } catch {
    // Private browsing can refuse writes. The token still works for this page
    // view; the person just gets a new one next time.
  }
}

function safeSessionStorage() {
  try {
    return globalThis.sessionStorage ?? null;
  } catch {
    return null;
  }
}
