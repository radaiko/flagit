import { describe, it, expect, vi } from 'vitest';
import { render, screen, waitFor } from '@testing-library/svelte';
import userEvent from '@testing-library/user-event';

import CreateTicket from '../src/overlay/CreateTicket.svelte';
import ViewTicket from '../src/overlay/ViewTicket.svelte';
import ReplyForm from '../src/overlay/ReplyForm.svelte';
import OverlayApp from '../src/overlay/App.svelte';
import { stubPublicClient, makeTicket, makeMessage, apiError } from './helpers.js';

const APP_INFO = {
  appName: 'notes',
  appVersion: '1.4.2',
  os: 'iOS 18.2',
  platform: 'ios',
  deviceModel: 'iPhone 15',
  logs: 'panic: nil map',
  logsDurationMin: 5,
};

describe('CreateTicket', () => {
  it('files a bug with the app diagnostics attached', async () => {
    const client = stubPublicClient();
    render(CreateTicket, { props: { client, lang: 'en', appInfo: APP_INFO } });

    await userEvent.type(screen.getByLabelText('Summary'), 'Crash on save');
    await userEvent.type(screen.getByLabelText('What happened'), 'It closes');
    await userEvent.click(screen.getByRole('button', { name: 'Send report' }));

    await waitFor(() => expect(client.createTicket).toHaveBeenCalled());
    expect(client.createTicket).toHaveBeenCalledWith(
      expect.objectContaining({
        type: 'bug',
        title: 'Crash on save',
        body: 'It closes',
        appName: 'notes',
        appVersion: '1.4.2',
        os: 'iOS 18.2',
        logs: 'panic: nil map',
        logsDurationMin: 5,
      }),
    );
  });

  it('switches the heading and placeholder for a feature request', async () => {
    render(CreateTicket, { props: { client: stubPublicClient(), lang: 'en' } });

    expect(screen.getByRole('heading', { name: 'What went wrong?' })).toBeInTheDocument();

    await userEvent.click(screen.getByText('Feature'));

    expect(screen.getByRole('heading', { name: 'What would you like to see?' })).toBeInTheDocument();
    expect(
      screen.getByPlaceholderText(/What you want to do, and why the app makes it hard/),
    ).toBeInTheDocument();
  });

  it('files a feature request as type feature', async () => {
    const client = stubPublicClient();
    render(CreateTicket, { props: { client, lang: 'en' } });

    await userEvent.click(screen.getByText('Feature'));
    await userEvent.type(screen.getByLabelText('Summary'), 'Dark mode');
    await userEvent.type(screen.getByLabelText('What happened'), 'Please');
    await userEvent.click(screen.getByRole('button', { name: 'Send report' }));

    await waitFor(() =>
      expect(client.createTicket).toHaveBeenCalledWith(
        expect.objectContaining({ type: 'feature' }),
      ),
    );
  });

  it('sends nothing diagnostic when the person opts out', async () => {
    const client = stubPublicClient();
    render(CreateTicket, { props: { client, lang: 'en', appInfo: APP_INFO } });

    await userEvent.click(screen.getByLabelText(/Attach diagnostics/));
    await userEvent.type(screen.getByLabelText('Summary'), 'Crash');
    await userEvent.type(screen.getByLabelText('What happened'), 'Boom');
    await userEvent.click(screen.getByRole('button', { name: 'Send report' }));

    await waitFor(() => expect(client.createTicket).toHaveBeenCalled());
    const sent = client.createTicket.mock.calls[0][0];
    expect(sent.logs).toBe('');
    expect(sent.logsDurationMin).toBe(0);
    // Opting out is about the logs, not about which app reported.
    expect(sent.appName).toBe('notes');
  });

  it('asks for a summary before sending', async () => {
    const client = stubPublicClient();
    render(CreateTicket, { props: { client, lang: 'en' } });

    await userEvent.click(screen.getByRole('button', { name: 'Send report' }));

    expect(await screen.findByRole('alert')).toHaveTextContent(/Add a short summary/);
    expect(client.createTicket).not.toHaveBeenCalled();
  });

  it('asks what happened before sending', async () => {
    const client = stubPublicClient();
    render(CreateTicket, { props: { client, lang: 'en' } });

    await userEvent.type(screen.getByLabelText('Summary'), 'Crash');
    await userEvent.click(screen.getByRole('button', { name: 'Send report' }));

    expect(await screen.findByRole('alert')).toHaveTextContent(/Describe what happened/);
    expect(client.createTicket).not.toHaveBeenCalled();
  });

  it('shows the ticket ID once the report is filed', async () => {
    const client = stubPublicClient();
    render(CreateTicket, { props: { client, lang: 'en' } });

    await userEvent.type(screen.getByLabelText('Summary'), 'Crash');
    await userEvent.type(screen.getByLabelText('What happened'), 'Boom');
    await userEvent.click(screen.getByRole('button', { name: 'Send report' }));

    expect(await screen.findByRole('heading', { name: 'Report filed' })).toBeInTheDocument();
    expect(screen.getByText('FLG-7X3K9Q')).toBeInTheDocument();
    expect(screen.getByText(/Keep this ID/)).toBeInTheDocument();
  });

  it('reports a server failure to the person', async () => {
    const client = stubPublicClient({
      createTicket: vi.fn().mockRejectedValue(apiError('error.network', 0)),
    });
    render(CreateTicket, { props: { client, lang: 'en' } });

    await userEvent.type(screen.getByLabelText('Summary'), 'Crash');
    await userEvent.type(screen.getByLabelText('What happened'), 'Boom');
    await userEvent.click(screen.getByRole('button', { name: 'Send report' }));

    expect(await screen.findByRole('alert')).toHaveTextContent(/Could not reach the server/);
  });

  it('falls back to a generic message for an unrecognised failure', async () => {
    const client = stubPublicClient({
      createTicket: vi.fn().mockRejectedValue(new Error('kaboom')),
    });
    render(CreateTicket, { props: { client, lang: 'en' } });

    await userEvent.type(screen.getByLabelText('Summary'), 'Crash');
    await userEvent.type(screen.getByLabelText('What happened'), 'Boom');
    await userEvent.click(screen.getByRole('button', { name: 'Send report' }));

    expect(await screen.findByRole('alert')).toHaveTextContent('Something went wrong. Try again.');
  });

  it('copies the ticket ID to the clipboard', async () => {
    const writeText = vi.fn().mockResolvedValue(undefined);
    vi.stubGlobal('navigator', { ...navigator, clipboard: { writeText } });

    render(CreateTicket, { props: { client: stubPublicClient(), lang: 'en' } });
    await fileATicket();

    await userEvent.click(screen.getByRole('button', { name: 'Copy ID' }));

    expect(writeText).toHaveBeenCalledWith('FLG-7X3K9Q');
    expect(await screen.findByRole('button', { name: 'Copied' })).toBeInTheDocument();
    vi.unstubAllGlobals();
  });

  it('stays quiet when the clipboard is blocked', async () => {
    vi.stubGlobal('navigator', {
      ...navigator,
      clipboard: { writeText: vi.fn().mockRejectedValue(new Error('denied')) },
    });

    render(CreateTicket, { props: { client: stubPublicClient(), lang: 'en' } });
    await fileATicket();

    await userEvent.click(screen.getByRole('button', { name: 'Copy ID' }));

    // The ID is on screen regardless, so there is nothing useful to say.
    expect(screen.getByRole('button', { name: 'Copy ID' })).toBeInTheDocument();
    vi.unstubAllGlobals();
  });

  it('resets to a blank form for another report', async () => {
    render(CreateTicket, { props: { client: stubPublicClient(), lang: 'en' } });
    await fileATicket();

    await userEvent.click(screen.getByRole('button', { name: 'File another report' }));

    expect(await screen.findByLabelText('Summary')).toHaveValue('');
    expect(screen.getByLabelText('What happened')).toHaveValue('');
  });

  it('hands the new ticket ID to the status screen', async () => {
    const oncheckstatus = vi.fn();
    render(CreateTicket, { props: { client: stubPublicClient(), lang: 'en', oncheckstatus } });
    await fileATicket();

    await userEvent.click(screen.getByRole('button', { name: 'Check status' }));

    expect(oncheckstatus).toHaveBeenCalledWith('FLG-7X3K9Q');
  });

  it('opens a blank lookup from the form', async () => {
    const oncheckstatus = vi.fn();
    render(CreateTicket, { props: { client: stubPublicClient(), lang: 'en', oncheckstatus } });

    await userEvent.click(screen.getByRole('button', { name: 'Look up a ticket' }));

    expect(oncheckstatus).toHaveBeenCalledWith('');
  });

  it('notifies the host when a ticket is filed', async () => {
    const onfiled = vi.fn();
    render(CreateTicket, { props: { client: stubPublicClient(), lang: 'en', onfiled } });

    await fileATicket();

    expect(onfiled).toHaveBeenCalledWith(expect.objectContaining({ id: 'FLG-7X3K9Q' }));
  });

  it('renders entirely in German', async () => {
    render(CreateTicket, { props: { client: stubPublicClient(), lang: 'de' } });

    expect(screen.getByRole('heading', { name: 'Was ist schiefgelaufen?' })).toBeInTheDocument();
    expect(screen.getByLabelText('Kurzfassung')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Meldung senden' })).toBeInTheDocument();
    expect(screen.getByText('Fehler')).toBeInTheDocument();
    expect(screen.getByText('Wunsch')).toBeInTheDocument();
  });

  it('shows German validation errors', async () => {
    render(CreateTicket, { props: { client: stubPublicClient(), lang: 'de' } });

    await userEvent.click(screen.getByRole('button', { name: 'Meldung senden' }));

    expect(await screen.findByRole('alert')).toHaveTextContent(
      'Schreib eine kurze Zusammenfassung, damit die Meldung auffindbar ist.',
    );
  });

  it('copes with no app info at all', async () => {
    const client = stubPublicClient();
    render(CreateTicket, { props: { client, lang: 'en' } });

    await fileATicket();

    expect(client.createTicket).toHaveBeenCalledWith(
      expect.objectContaining({ appName: '', appVersion: '', logs: '', logsDurationMin: 0 }),
    );
  });
});

describe('ViewTicket', () => {
  it('opens a ticket by ID', async () => {
    const client = stubPublicClient();
    render(ViewTicket, { props: { client, lang: 'en' } });

    await userEvent.type(screen.getByLabelText('Ticket ID'), 'flg-7x3k9q');
    await userEvent.click(screen.getByRole('button', { name: 'Open ticket' }));

    await waitFor(() => expect(client.getTicket).toHaveBeenCalledWith('FLG-7X3K9Q'));
    expect(await screen.findByRole('heading', { name: 'Crash on save' })).toBeInTheDocument();
  });

  it('opens straight away when handed an ID', async () => {
    const client = stubPublicClient();
    render(ViewTicket, { props: { client, lang: 'en', initialId: 'FLG-7X3K9Q' } });

    await waitFor(() => expect(client.getTicket).toHaveBeenCalledWith('FLG-7X3K9Q'));
    expect(await screen.findByRole('heading', { name: 'Crash on save' })).toBeInTheDocument();
  });

  it('asks for an ID when the field is empty', async () => {
    const client = stubPublicClient();
    render(ViewTicket, { props: { client, lang: 'en' } });

    await userEvent.click(screen.getByRole('button', { name: 'Open ticket' }));

    expect(await screen.findByRole('alert')).toHaveTextContent('Enter a ticket ID.');
    expect(client.getTicket).not.toHaveBeenCalled();
  });

  it('explains a ticket that belongs to another device', async () => {
    const client = stubPublicClient({
      getTicket: vi.fn().mockRejectedValue(apiError('error.forbidden', 403)),
    });
    render(ViewTicket, { props: { client, lang: 'en' } });

    await userEvent.type(screen.getByLabelText('Ticket ID'), 'FLG-OTHER1');
    await userEvent.click(screen.getByRole('button', { name: 'Open ticket' }));

    expect(await screen.findByRole('alert')).toHaveTextContent(/belongs to another device/);
  });

  it('reports an unknown ticket', async () => {
    const client = stubPublicClient({
      getTicket: vi.fn().mockRejectedValue(apiError('error.notFound', 404)),
    });
    render(ViewTicket, { props: { client, lang: 'en' } });

    await userEvent.type(screen.getByLabelText('Ticket ID'), 'FLG-ZZZZZZ');
    await userEvent.click(screen.getByRole('button', { name: 'Open ticket' }));

    expect(await screen.findByRole('alert')).toHaveTextContent('No ticket with that ID.');
  });

  it('falls back to a generic error', async () => {
    const client = stubPublicClient({
      getTicket: vi.fn().mockRejectedValue(new Error('kaboom')),
    });
    render(ViewTicket, { props: { client, lang: 'en' } });

    await userEvent.type(screen.getByLabelText('Ticket ID'), 'FLG-7X3K9Q');
    await userEvent.click(screen.getByRole('button', { name: 'Open ticket' }));

    expect(await screen.findByRole('alert')).toHaveTextContent('Something went wrong. Try again.');
  });

  it('shows the status, dates and conversation', async () => {
    const client = stubPublicClient({
      getTicket: vi.fn().mockResolvedValue(
        makeTicket({
          status: 'in-progress',
          messages: [makeMessage({ id: 1, role: 'agent', body: 'Reproduced it' })],
        }),
      ),
    });
    render(ViewTicket, { props: { client, lang: 'en', initialId: 'FLG-7X3K9Q' } });

    expect(await screen.findByText('In progress')).toBeInTheDocument();
    expect(screen.getByText('Reproduced it')).toBeInTheDocument();
    expect(screen.getByText('20 Jul 2026')).toBeInTheDocument();
  });

  it('shows the version a fix shipped in', async () => {
    const client = stubPublicClient({
      getTicket: vi.fn().mockResolvedValue(makeTicket({ status: 'shipped', shippedVersion: '1.5.0' })),
    });
    render(ViewTicket, { props: { client, lang: 'en', initialId: 'FLG-7X3K9Q' } });

    expect(await screen.findByText('1.5.0')).toBeInTheDocument();
  });

  it('appends a reply without refetching the ticket', async () => {
    const client = stubPublicClient();
    render(ViewTicket, { props: { client, lang: 'en', initialId: 'FLG-7X3K9Q' } });
    await screen.findByRole('heading', { name: 'Crash on save' });
    expect(client.getTicket).toHaveBeenCalledTimes(1);

    await userEvent.type(screen.getByLabelText('Add to this report'), 'Still broken');
    await userEvent.click(screen.getByRole('button', { name: 'Send reply' }));

    expect(await screen.findByText('Still broken')).toBeInTheDocument();
    expect(client.getTicket).toHaveBeenCalledTimes(1);
  });

  it('goes back', async () => {
    const onback = vi.fn();
    render(ViewTicket, { props: { client: stubPublicClient(), lang: 'en', onback } });

    await userEvent.click(screen.getByRole('button', { name: 'Back' }));

    expect(onback).toHaveBeenCalled();
  });

  it('goes back from an open ticket', async () => {
    const onback = vi.fn();
    render(ViewTicket, {
      props: { client: stubPublicClient(), lang: 'en', initialId: 'FLG-7X3K9Q', onback },
    });
    await screen.findByRole('heading', { name: 'Crash on save' });

    await userEvent.click(screen.getByRole('button', { name: 'Back' }));

    expect(onback).toHaveBeenCalled();
  });

  it('renders in German', async () => {
    render(ViewTicket, { props: { client: stubPublicClient(), lang: 'de' } });

    expect(screen.getByRole('heading', { name: 'Ticket nachschlagen' })).toBeInTheDocument();
    expect(screen.getByLabelText('Ticket-ID')).toBeInTheDocument();
  });
});

describe('ReplyForm', () => {
  it('posts a reply and clears the field', async () => {
    const client = stubPublicClient();
    render(ReplyForm, { props: { client, ticketId: 'FLG-7X3K9Q', lang: 'en' } });

    const field = screen.getByLabelText('Add to this report');
    await userEvent.type(field, '  Still broken  ');
    await userEvent.click(screen.getByRole('button', { name: 'Send reply' }));

    await waitFor(() =>
      expect(client.postMessage).toHaveBeenCalledWith('FLG-7X3K9Q', 'Still broken'),
    );
    expect(field).toHaveValue('');
    expect(await screen.findByRole('status')).toHaveTextContent('Reply sent');
  });

  it('will not send an empty reply', async () => {
    const client = stubPublicClient();
    render(ReplyForm, { props: { client, ticketId: 'FLG-7X3K9Q', lang: 'en' } });

    await userEvent.click(screen.getByRole('button', { name: 'Send reply' }));

    expect(await screen.findByRole('alert')).toHaveTextContent('Write a reply before sending.');
    expect(client.postMessage).not.toHaveBeenCalled();
  });

  it('reports a failure', async () => {
    const client = stubPublicClient({
      postMessage: vi.fn().mockRejectedValue(apiError('error.forbidden', 403)),
    });
    render(ReplyForm, { props: { client, ticketId: 'FLG-7X3K9Q', lang: 'en' } });

    await userEvent.type(screen.getByLabelText('Add to this report'), 'Hello');
    await userEvent.click(screen.getByRole('button', { name: 'Send reply' }));

    expect(await screen.findByRole('alert')).toHaveTextContent(/belongs to another device/);
  });

  it('falls back to a generic error', async () => {
    const client = stubPublicClient({
      postMessage: vi.fn().mockRejectedValue(new Error('kaboom')),
    });
    render(ReplyForm, { props: { client, ticketId: 'FLG-7X3K9Q', lang: 'en' } });

    await userEvent.type(screen.getByLabelText('Add to this report'), 'Hello');
    await userEvent.click(screen.getByRole('button', { name: 'Send reply' }));

    expect(await screen.findByRole('alert')).toHaveTextContent('Something went wrong. Try again.');
  });

  it('hands the new message to its parent', async () => {
    const onsent = vi.fn();
    render(ReplyForm, {
      props: { client: stubPublicClient(), ticketId: 'FLG-7X3K9Q', lang: 'en', onsent },
    });

    await userEvent.type(screen.getByLabelText('Add to this report'), 'Hello');
    await userEvent.click(screen.getByRole('button', { name: 'Send reply' }));

    await waitFor(() => expect(onsent).toHaveBeenCalled());
  });
});

describe('overlay App', () => {
  it('starts on the report form', () => {
    render(OverlayApp, { props: { client: stubPublicClient(), lang: 'en' } });

    expect(screen.getByRole('heading', { name: 'What went wrong?' })).toBeInTheDocument();
  });

  it('moves to the ticket view after filing', async () => {
    render(OverlayApp, { props: { client: stubPublicClient(), lang: 'en' } });

    await userEvent.type(screen.getByLabelText('Summary'), 'Crash');
    await userEvent.type(screen.getByLabelText('What happened'), 'Boom');
    await userEvent.click(screen.getByRole('button', { name: 'Send report' }));
    await screen.findByRole('heading', { name: 'Report filed' });

    await userEvent.click(screen.getByRole('button', { name: 'Check status' }));

    expect(await screen.findByRole('heading', { name: 'Crash on save' })).toBeInTheDocument();
  });

  it('returns to the form from the ticket view', async () => {
    render(OverlayApp, { props: { client: stubPublicClient(), lang: 'en', screen: 'view' } });

    await userEvent.click(screen.getByRole('button', { name: 'Back' }));

    expect(await screen.findByRole('heading', { name: 'What went wrong?' })).toBeInTheDocument();
  });

  it('switches language across the whole overlay', async () => {
    render(OverlayApp, { props: { client: stubPublicClient(), lang: 'en' } });
    expect(screen.getByRole('heading', { name: 'What went wrong?' })).toBeInTheDocument();

    await userEvent.click(screen.getByRole('button', { name: /Auf Deutsch umschalten/ }));

    expect(await screen.findByRole('heading', { name: 'Was ist schiefgelaufen?' })).toBeInTheDocument();
    expect(sessionStorage.getItem('flagit.lang')).toBe('de');
  });

  it('builds its own client from the device token when none is given', () => {
    render(OverlayApp, { props: { lang: 'en' } });

    expect(screen.getByRole('heading', { name: 'What went wrong?' })).toBeInTheDocument();
    expect(sessionStorage.getItem('flagit.deviceToken')).toBeTruthy();
  });

  it('detects the language when none is given', () => {
    sessionStorage.setItem('flagit.lang', 'de');

    render(OverlayApp, { props: { client: stubPublicClient() } });

    expect(screen.getByRole('heading', { name: 'Was ist schiefgelaufen?' })).toBeInTheDocument();
  });
});

/** File a valid report through the currently rendered CreateTicket form. */
async function fileATicket() {
  await userEvent.type(screen.getByLabelText(/Summary|Kurzfassung/), 'Crash');
  await userEvent.type(screen.getByLabelText(/What happened|Was passiert ist/), 'Boom');
  await userEvent.click(screen.getByRole('button', { name: /Send report|Meldung senden/ }));
  await screen.findByRole('heading', { name: /Report filed|Meldung ist eingegangen/ });
}
