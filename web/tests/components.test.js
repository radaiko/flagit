import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/svelte';
import userEvent from '@testing-library/user-event';

import FlagTag from '../src/lib/FlagTag.svelte';
import StatusLabel from '../src/lib/StatusLabel.svelte';
import MessageList from '../src/lib/MessageList.svelte';
import LanguageToggle from '../src/lib/LanguageToggle.svelte';
import { makeMessage } from './helpers.js';

describe('FlagTag', () => {
  it('shows the ticket ID', () => {
    render(FlagTag, { props: { id: 'FLG-7X3K9Q', status: 'open' } });

    expect(screen.getByText('FLG-7X3K9Q')).toBeInTheDocument();
  });

  it('carries the status so the colour bar can key off it', () => {
    const { container } = render(FlagTag, { props: { id: 'FLG-7X3K9Q', status: 'shipped' } });

    expect(container.querySelector('.tag')).toHaveAttribute('data-status', 'shipped');
  });

  it('names the status for screen readers, since colour alone is not enough', () => {
    render(FlagTag, { props: { id: 'FLG-7X3K9Q', status: 'in-progress', lang: 'en' } });

    expect(screen.getByText(/Status: In progress/)).toBeInTheDocument();
  });

  it('names the status in German too', () => {
    render(FlagTag, { props: { id: 'FLG-7X3K9Q', status: 'shipped', lang: 'de' } });

    expect(screen.getByText(/Status: Ausgeliefert/)).toBeInTheDocument();
  });

  it('defaults to a small open tag', () => {
    const { container } = render(FlagTag, { props: { id: 'FLG-7X3K9Q' } });

    const tag = container.querySelector('.tag');
    expect(tag).toHaveClass('sm');
    expect(tag).toHaveAttribute('data-status', 'open');
  });

  it('renders at the large size for the success screen', () => {
    const { container } = render(FlagTag, { props: { id: 'FLG-7X3K9Q', size: 'lg' } });

    expect(container.querySelector('.tag')).toHaveClass('lg');
  });
});

describe('StatusLabel', () => {
  it.each([
    ['open', 'Open', 'Offen'],
    ['in-progress', 'In progress', 'In Arbeit'],
    ['resolved', 'Resolved', 'Behoben'],
    ['shipped', 'Shipped', 'Ausgeliefert'],
    ['closed', 'Closed', 'Geschlossen'],
  ])('renders %s in both languages', (status, english, german) => {
    const { unmount } = render(StatusLabel, { props: { status, lang: 'en' } });
    expect(screen.getByText(english)).toBeInTheDocument();
    unmount();

    render(StatusLabel, { props: { status, lang: 'de' } });
    expect(screen.getByText(german)).toBeInTheDocument();
  });

  it('exposes the status for styling', () => {
    const { container } = render(StatusLabel, { props: { status: 'resolved' } });

    expect(container.querySelector('.status')).toHaveAttribute('data-status', 'resolved');
  });
});

describe('MessageList', () => {
  it('invites the reader to wait when there is nothing yet', () => {
    render(MessageList, { props: { messages: [], lang: 'en' } });

    expect(screen.getByText(/No replies yet/)).toBeInTheDocument();
  });

  it('uses a caller-supplied empty message', () => {
    render(MessageList, { props: { messages: [], lang: 'en', emptyKey: 'detail.noCommits' } });

    expect(screen.getByText('No commits recorded yet.')).toBeInTheDocument();
  });

  it('lists messages with their author and time', () => {
    render(MessageList, {
      props: {
        lang: 'en',
        messages: [
          makeMessage({ id: 1, role: 'user', body: 'Still broken' }),
          makeMessage({ id: 2, role: 'agent', body: 'Fix is out' }),
        ],
      },
    });

    expect(screen.getByText('Still broken')).toBeInTheDocument();
    expect(screen.getByText('Fix is out')).toBeInTheDocument();
    expect(screen.getByText('You')).toBeInTheDocument();
    expect(screen.getByText('Flagit')).toBeInTheDocument();
  });

  it('marks which side each message came from', () => {
    const { container } = render(MessageList, {
      props: {
        messages: [makeMessage({ id: 1, role: 'user' }), makeMessage({ id: 2, role: 'agent' })],
      },
    });

    const entries = container.querySelectorAll('.entry');
    expect(entries[0]).toHaveAttribute('data-role', 'user');
    expect(entries[1]).toHaveAttribute('data-role', 'agent');
  });

  it('translates the authors', () => {
    render(MessageList, {
      props: { lang: 'de', messages: [makeMessage({ id: 1, role: 'user' })] },
    });

    expect(screen.getByText('Du')).toBeInTheDocument();
  });

  it('defaults to no messages', () => {
    render(MessageList, { props: {} });

    expect(screen.getByText(/No replies yet/)).toBeInTheDocument();
  });
});

describe('LanguageToggle', () => {
  it('shows both languages and marks the active one', () => {
    const { container } = render(LanguageToggle, { props: { lang: 'en' } });

    expect(screen.getByText('EN')).toHaveClass('active');
    expect(screen.getByText('DE')).not.toHaveClass('active');
    expect(container.querySelector('button')).toHaveAttribute('title', 'Auf Deutsch umschalten');
  });

  it('offers the other language when German is active', () => {
    const { container } = render(LanguageToggle, { props: { lang: 'de' } });

    expect(screen.getByText('DE')).toHaveClass('active');
    expect(container.querySelector('button')).toHaveAttribute('title', 'Switch to English');
  });

  it('reports the language to switch to', async () => {
    const onchange = vi.fn();
    render(LanguageToggle, { props: { lang: 'en', onchange } });

    await userEvent.click(screen.getByRole('button'));

    expect(onchange).toHaveBeenCalledWith('de');
  });

  it('flips back the other way', async () => {
    const onchange = vi.fn();
    render(LanguageToggle, { props: { lang: 'de', onchange } });

    await userEvent.click(screen.getByRole('button'));

    expect(onchange).toHaveBeenCalledWith('en');
  });

  it('survives being rendered without a handler', async () => {
    render(LanguageToggle, { props: { lang: 'en' } });

    await expect(userEvent.click(screen.getByRole('button'))).resolves.not.toThrow();
  });
});
