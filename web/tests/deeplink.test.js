import { describe, it, expect } from 'vitest';
import { resolveTicketId } from '../src/lib/deeplink.js';

describe('resolveTicketId', () => {
  it('reads the ticket a host application handed over', () => {
    expect(resolveTicketId({ search: '?ticket=FLG-7X3K9Q' })).toBe('FLG-7X3K9Q');
  });

  it('reads it alongside the token, which is the way it actually arrives', () => {
    expect(resolveTicketId({ search: '?token=abc-123&ticket=FLG-7X3K9Q' })).toBe('FLG-7X3K9Q');
  });

  // Base62: FLG-7x3k9q and FLG-7X3K9Q are different tickets, and IDs minted
  // before the alphabet was narrowed still carry lowercase.
  it('never re-cases the ID', () => {
    expect(resolveTicketId({ search: '?ticket=FLG-peJDtJ' })).toBe('FLG-peJDtJ');
  });

  it('forgives the whitespace that survives a copy and paste', () => {
    expect(resolveTicketId({ search: '?ticket=%20FLG-7X3K9Q%20' })).toBe('FLG-7X3K9Q');
  });

  it('has nothing to say when no ticket was named', () => {
    expect(resolveTicketId({ search: '' })).toBe('');
    expect(resolveTicketId({ search: '?token=abc-123' })).toBe('');
    expect(resolveTicketId({ search: '?ticket=' })).toBe('');
  });

  // The value comes out of a URL anyone can edit. There is nothing to look up
  // for a malformed one, and the lookup field is a better answer than an error.
  it('treats anything that is not an ID as no ID', () => {
    expect(resolveTicketId({ search: '?ticket=nonsense' })).toBe('');
    expect(resolveTicketId({ search: '?ticket=FLG-' })).toBe('');
    expect(resolveTicketId({ search: '?ticket=FLG-abc' })).toBe('');
    expect(resolveTicketId({ search: '?ticket=FLG-0123456789abc' })).toBe('');
    expect(resolveTicketId({ search: '?ticket=<script>alert(1)</script>' })).toBe('');
  });
});
