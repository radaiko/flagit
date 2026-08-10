import { describe, it, expect, vi } from 'vitest';
import { render, screen, waitFor, within } from '@testing-library/svelte';
import userEvent from '@testing-library/user-event';

import Login from '../src/dashboard/Login.svelte';
import TicketList from '../src/dashboard/TicketList.svelte';
import TicketDetail from '../src/dashboard/TicketDetail.svelte';
import MassOperations from '../src/dashboard/MassOperations.svelte';
import Settings from '../src/dashboard/Settings.svelte';
import BuildInfo from '../src/dashboard/BuildInfo.svelte';
import DashboardApp from '../src/dashboard/App.svelte';
import { createAdminClient } from '../src/lib/api.js';
import { stubAdminClient, makeTicket, makeMessage, makeApp, apiError } from './helpers.js';

describe('Login', () => {
  it('verifies the key against the API before letting anyone in', async () => {
    const client = stubAdminClient();
    const clientFactory = vi.fn().mockReturnValue(client);
    const onauthenticated = vi.fn();
    render(Login, { props: { lang: 'en', clientFactory, onauthenticated } });

    await userEvent.type(screen.getByLabelText('Admin key'), '  secret-key  ');
    await userEvent.click(screen.getByRole('button', { name: 'Sign in' }));

    await waitFor(() => expect(onauthenticated).toHaveBeenCalled());
    expect(clientFactory).toHaveBeenCalledWith({ adminKey: 'secret-key' });
    expect(client.getSettings).toHaveBeenCalled();
    expect(onauthenticated).toHaveBeenCalledWith('secret-key', client);
  });

  it('asks for a key when the field is empty', async () => {
    const clientFactory = vi.fn();
    render(Login, { props: { lang: 'en', clientFactory } });

    await userEvent.click(screen.getByRole('button', { name: 'Sign in' }));

    expect(await screen.findByRole('alert')).toHaveTextContent('Enter the admin key.');
    expect(clientFactory).not.toHaveBeenCalled();
  });

  it('says the key was rejected rather than letting it fail later', async () => {
    const clientFactory = vi.fn().mockReturnValue(
      stubAdminClient({
        getSettings: vi.fn().mockRejectedValue(apiError('admin.keyInvalid', 401)),
      }),
    );
    const onauthenticated = vi.fn();
    render(Login, { props: { lang: 'en', clientFactory, onauthenticated } });

    await userEvent.type(screen.getByLabelText('Admin key'), 'wrong-key');
    await userEvent.click(screen.getByRole('button', { name: 'Sign in' }));

    expect(await screen.findByRole('alert')).toHaveTextContent(/key was rejected/);
    expect(onauthenticated).not.toHaveBeenCalled();
  });

  it('falls back to a generic error', async () => {
    const clientFactory = vi.fn().mockReturnValue(
      stubAdminClient({ getSettings: vi.fn().mockRejectedValue(new Error('kaboom')) }),
    );
    render(Login, { props: { lang: 'en', clientFactory } });

    await userEvent.type(screen.getByLabelText('Admin key'), 'key');
    await userEvent.click(screen.getByRole('button', { name: 'Sign in' }));

    expect(await screen.findByRole('alert')).toHaveTextContent('Something went wrong. Try again.');
  });

  it('masks the key as it is typed', () => {
    render(Login, { props: { lang: 'en' } });

    expect(screen.getByLabelText('Admin key')).toHaveAttribute('type', 'password');
  });

  it('renders in German', () => {
    render(Login, { props: { lang: 'de' } });

    expect(screen.getByLabelText('Admin-Schlüssel')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Anmelden' })).toBeInTheDocument();
  });
});

describe('TicketList', () => {
  it('lists tickets', async () => {
    render(TicketList, { props: { client: stubAdminClient(), lang: 'en' } });

    expect(await screen.findByText('Crash on save')).toBeInTheDocument();
    expect(screen.getByText('FLG-7X3K9Q')).toBeInTheDocument();
    // Scoped to the table: 'Open' is also a filter option.
    expect(within(screen.getByRole('table')).getByText('Open')).toBeInTheDocument();
  });

  it('invites action when there is nothing yet', async () => {
    const client = stubAdminClient({ listTickets: vi.fn().mockResolvedValue([]) });
    render(TicketList, { props: { client, lang: 'en' } });

    expect(await screen.findByText(/No tickets yet/)).toBeInTheDocument();
  });

  /*
   * "No tickets yet" is a statement about the database, and this is the case
   * where it was a lie: a misrouted admin listener answered the ticket query
   * with the dashboard's own HTML, the client read no tickets out of it, and
   * the table said the tracker was empty while the tickets sat in SQLite.
   *
   * Whatever else this screen does with a broken listener, it must not claim
   * to have looked.
   */
  it('reports a listener it cannot read, rather than calling the tracker empty', async () => {
    const client = stubAdminClient({
      listTickets: vi.fn().mockRejectedValue(apiError('error.malformedResponse', 200)),
    });
    render(TicketList, { props: { client, lang: 'en' } });

    expect(await screen.findByRole('alert')).toHaveTextContent(/could not read/i);
    expect(screen.queryByText(/No tickets yet/)).not.toBeInTheDocument();
  });

  it('distinguishes an empty result from an empty filter', async () => {
    const client = stubAdminClient({
      listTickets: vi.fn().mockResolvedValue([makeTicket({ appName: 'notes' })]),
      listApps: vi.fn().mockResolvedValue([makeApp({ name: 'notes' }), makeApp({ name: 'timer' })]),
    });
    render(TicketList, { props: { client, lang: 'en' } });
    await screen.findByText('Crash on save');

    await userEvent.selectOptions(screen.getByLabelText('App'), 'timer');

    expect(await screen.findByText('No tickets match these filters.')).toBeInTheDocument();
  });

  it('filters by app, status and type', async () => {
    const client = stubAdminClient({
      listTickets: vi.fn().mockResolvedValue([
        makeTicket({ id: 'FLG-AAAAAA', appName: 'notes', status: 'open', type: 'bug', title: 'Alpha' }),
        makeTicket({ id: 'FLG-BBBBBB', appName: 'timer', status: 'shipped', type: 'feature', title: 'Beta' }),
      ]),
      listApps: vi.fn().mockResolvedValue([makeApp({ name: 'notes' }), makeApp({ name: 'timer' })]),
    });
    render(TicketList, { props: { client, lang: 'en' } });
    await screen.findByText('Alpha');

    await userEvent.selectOptions(screen.getByLabelText('App'), 'timer');
    expect(screen.queryByText('Alpha')).not.toBeInTheDocument();
    expect(screen.getByText('Beta')).toBeInTheDocument();

    await userEvent.selectOptions(screen.getByLabelText('App'), '');
    await userEvent.selectOptions(screen.getByLabelText('Status'), 'open');
    expect(screen.getByText('Alpha')).toBeInTheDocument();
    expect(screen.queryByText('Beta')).not.toBeInTheDocument();

    await userEvent.selectOptions(screen.getByLabelText('Status'), '');
    await userEvent.selectOptions(screen.getByLabelText('Type'), 'feature');
    expect(screen.queryByText('Alpha')).not.toBeInTheDocument();
    expect(screen.getByText('Beta')).toBeInTheDocument();
  });

  // Triage is where declined earns its keep: an admin has to be able to pull
  // up everything that was turned down without it hiding among the closed.
  it('offers declined in the status filter and narrows to it', async () => {
    const client = stubAdminClient({
      listTickets: vi.fn().mockResolvedValue([
        makeTicket({ id: 'FLG-AAAAAA', status: 'closed', title: 'Alpha' }),
        makeTicket({ id: 'FLG-CCCCCC', status: 'declined', title: 'Gamma' }),
      ]),
    });
    render(TicketList, { props: { client, lang: 'en' } });
    await screen.findByText('Gamma');

    const filter = screen.getByLabelText('Status');
    expect(within(filter).getByRole('option', { name: 'Declined' })).toBeInTheDocument();

    await userEvent.selectOptions(filter, 'declined');

    expect(screen.getByText('Gamma')).toBeInTheDocument();
    expect(screen.queryByText('Alpha')).not.toBeInTheDocument();
  });

  it('names a declined ticket in the table, in German too', async () => {
    const client = stubAdminClient({
      listTickets: vi.fn().mockResolvedValue([makeTicket({ status: 'declined' })]),
    });
    render(TicketList, { props: { client, lang: 'de' } });

    await screen.findByText('Crash on save');
    expect(within(screen.getByRole('table')).getByText('Abgelehnt')).toBeInTheDocument();
  });

  it('opens a ticket when its tag is clicked', async () => {
    const onopen = vi.fn();
    render(TicketList, { props: { client: stubAdminClient(), lang: 'en', onopen } });
    await screen.findByText('Crash on save');

    await userEvent.click(screen.getByText('FLG-7X3K9Q'));

    expect(onopen).toHaveBeenCalledWith('FLG-7X3K9Q');
  });

  it('reveals bulk operations once something is selected', async () => {
    render(TicketList, { props: { client: stubAdminClient(), lang: 'en' } });
    await screen.findByText('Crash on save');
    expect(screen.queryByText(/selected/)).not.toBeInTheDocument();

    await userEvent.click(screen.getByLabelText('Select ticket FLG-7X3K9Q'));

    expect(await screen.findByText('selected')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Apply' })).toBeInTheDocument();
  });

  it('selects and clears everything at once', async () => {
    const client = stubAdminClient({
      listTickets: vi
        .fn()
        .mockResolvedValue([makeTicket({ id: 'FLG-AAAAAA' }), makeTicket({ id: 'FLG-BBBBBB' })]),
    });
    render(TicketList, { props: { client, lang: 'en' } });
    await screen.findByText('FLG-AAAAAA');

    await userEvent.click(screen.getByLabelText('Select all'));
    expect(await screen.findByText('2')).toBeInTheDocument();

    await userEvent.click(screen.getByLabelText('Select all'));
    await waitFor(() => expect(screen.queryByText('selected')).not.toBeInTheDocument());
  });

  it('deselects one ticket without losing the rest', async () => {
    const client = stubAdminClient({
      listTickets: vi
        .fn()
        .mockResolvedValue([makeTicket({ id: 'FLG-AAAAAA' }), makeTicket({ id: 'FLG-BBBBBB' })]),
    });
    render(TicketList, { props: { client, lang: 'en' } });
    await screen.findByText('FLG-AAAAAA');

    await userEvent.click(screen.getByLabelText('Select all'));
    await userEvent.click(screen.getByLabelText('Select ticket FLG-AAAAAA'));

    expect(await screen.findByText('1')).toBeInTheDocument();
  });

  it('clears the selection from the bulk panel', async () => {
    render(TicketList, { props: { client: stubAdminClient(), lang: 'en' } });
    await screen.findByText('Crash on save');
    await userEvent.click(screen.getByLabelText('Select ticket FLG-7X3K9Q'));
    await screen.findByText('selected');

    await userEvent.click(screen.getByRole('button', { name: 'Clear selection' }));

    await waitFor(() => expect(screen.queryByText('selected')).not.toBeInTheDocument());
  });

  it('reloads and drops the selection after a bulk update', async () => {
    const client = stubAdminClient();
    render(TicketList, { props: { client, lang: 'en' } });
    await screen.findByText('Crash on save');
    await userEvent.click(screen.getByLabelText('Select ticket FLG-7X3K9Q'));
    await screen.findByText('selected');

    await userEvent.click(screen.getByRole('button', { name: 'Apply' }));

    await waitFor(() => expect(client.batchUpdate).toHaveBeenCalled());
    await waitFor(() => expect(client.listTickets).toHaveBeenCalledTimes(2));
    await waitFor(() => expect(screen.queryByText('selected')).not.toBeInTheDocument());
  });

  it('refreshes on demand', async () => {
    const client = stubAdminClient();
    render(TicketList, { props: { client, lang: 'en' } });
    await screen.findByText('Crash on save');

    await userEvent.click(screen.getByRole('button', { name: 'Refresh' }));

    await waitFor(() => expect(client.listTickets).toHaveBeenCalledTimes(2));
  });

  it('reports a failure to load', async () => {
    const client = stubAdminClient({
      listTickets: vi.fn().mockRejectedValue(apiError('error.network', 0)),
    });
    render(TicketList, { props: { client, lang: 'en' } });

    expect(await screen.findByRole('alert')).toHaveTextContent(/Could not reach the server/);
  });

  it('falls back to a generic load error', async () => {
    const client = stubAdminClient({
      listTickets: vi.fn().mockRejectedValue(new Error('kaboom')),
    });
    render(TicketList, { props: { client, lang: 'en' } });

    expect(await screen.findByRole('alert')).toHaveTextContent('Something went wrong. Try again.');
  });

  it('copes with an API that returns nothing', async () => {
    const client = stubAdminClient({
      listTickets: vi.fn().mockResolvedValue(null),
      listApps: vi.fn().mockResolvedValue(null),
    });
    render(TicketList, { props: { client, lang: 'en' } });

    expect(await screen.findByText(/No tickets yet/)).toBeInTheDocument();
  });

  it('renders in German', async () => {
    render(TicketList, { props: { client: stubAdminClient(), lang: 'de' } });

    expect(await screen.findByRole('heading', { name: 'Tickets' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Aktualisieren' })).toBeInTheDocument();
    expect(within(screen.getByRole('table')).getByText('Offen')).toBeInTheDocument();
  });
});

describe('MassOperations', () => {
  const selected = ['FLG-AAAAAA', 'FLG-BBBBBB'];

  it('defaults to shipping and asks for a version', () => {
    render(MassOperations, { props: { client: stubAdminClient(), selected, lang: 'en' } });

    expect(screen.getByLabelText('Set status to')).toHaveValue('shipped');
    expect(screen.getByLabelText('Shipped in version')).toBeInTheDocument();
  });

  it('hides the version field for statuses where it means nothing', async () => {
    render(MassOperations, { props: { client: stubAdminClient(), selected, lang: 'en' } });

    await userEvent.selectOptions(screen.getByLabelText('Set status to'), 'closed');

    expect(screen.queryByLabelText('Shipped in version')).not.toBeInTheDocument();
  });

  // Turning down a batch of requests in one sweep is the mirror image of
  // shipping one, so the bulk control has to offer it.
  it('can decline a whole selection at once', async () => {
    const client = stubAdminClient();
    render(MassOperations, { props: { client, selected, lang: 'en' } });

    const select = screen.getByLabelText('Set status to');
    expect(within(select).getByRole('option', { name: 'Declined' })).toBeInTheDocument();

    await userEvent.selectOptions(select, 'declined');
    expect(screen.queryByLabelText('Shipped in version')).not.toBeInTheDocument();

    await userEvent.click(screen.getByRole('button', { name: 'Apply' }));

    await waitFor(() =>
      expect(client.batchUpdate).toHaveBeenCalledWith(selected, 'declined', ''),
    );
  });

  it('applies the update to every selected ticket', async () => {
    const client = stubAdminClient({
      batchUpdate: vi.fn().mockResolvedValue({ updated: selected, failed: {} }),
    });
    render(MassOperations, { props: { client, selected, lang: 'en' } });

    await userEvent.type(screen.getByLabelText('Shipped in version'), '  1.5.0  ');
    await userEvent.click(screen.getByRole('button', { name: 'Apply' }));

    await waitFor(() => expect(client.batchUpdate).toHaveBeenCalledWith(selected, 'shipped', '1.5.0'));
    expect(await screen.findByRole('status')).toHaveTextContent('Updated 2 tickets');
  });

  it('says how many could not be updated', async () => {
    const client = stubAdminClient({
      batchUpdate: vi.fn().mockResolvedValue({
        updated: ['FLG-AAAAAA'],
        failed: { 'FLG-BBBBBB': 'not found' },
      }),
    });
    render(MassOperations, { props: { client, selected, lang: 'en' } });

    await userEvent.click(screen.getByRole('button', { name: 'Apply' }));

    expect(await screen.findByRole('status')).toHaveTextContent(
      'Updated 1 tickets, 1 could not be updated',
    );
  });

  it('copes with a response that omits the failure map', async () => {
    const client = stubAdminClient({
      batchUpdate: vi.fn().mockResolvedValue({ updated: ['FLG-AAAAAA'] }),
    });
    render(MassOperations, { props: { client, selected, lang: 'en' } });

    await userEvent.click(screen.getByRole('button', { name: 'Apply' }));

    expect(await screen.findByRole('status')).toHaveTextContent('Updated 1 tickets');
  });

  it('reports a failure', async () => {
    const client = stubAdminClient({
      batchUpdate: vi.fn().mockRejectedValue(apiError('error.generic', 500)),
    });
    render(MassOperations, { props: { client, selected, lang: 'en' } });

    await userEvent.click(screen.getByRole('button', { name: 'Apply' }));

    expect(await screen.findByRole('alert')).toHaveTextContent('Something went wrong. Try again.');
  });

  it('falls back to a generic error', async () => {
    const client = stubAdminClient({
      batchUpdate: vi.fn().mockRejectedValue(new Error('kaboom')),
    });
    render(MassOperations, { props: { client, selected, lang: 'en' } });

    await userEvent.click(screen.getByRole('button', { name: 'Apply' }));

    expect(await screen.findByRole('alert')).toHaveTextContent('Something went wrong. Try again.');
  });

  it('clears the selection', async () => {
    const onclear = vi.fn();
    render(MassOperations, { props: { client: stubAdminClient(), selected, lang: 'en', onclear } });

    await userEvent.click(screen.getByRole('button', { name: 'Clear selection' }));

    expect(onclear).toHaveBeenCalled();
  });

  it('survives having no handlers', async () => {
    render(MassOperations, { props: { client: stubAdminClient(), selected, lang: 'en' } });

    await userEvent.click(screen.getByRole('button', { name: 'Apply' }));
    await userEvent.click(screen.getByRole('button', { name: 'Clear selection' }));

    expect(await screen.findByRole('status')).toBeInTheDocument();
  });

  it('renders in German', () => {
    render(MassOperations, { props: { client: stubAdminClient(), selected, lang: 'de' } });

    expect(screen.getByLabelText('Status setzen auf')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Übernehmen' })).toBeInTheDocument();
  });
});

describe('TicketDetail', () => {
  const fullTicket = makeTicket({
    status: 'in-progress',
    logs: 'panic: nil map\n\tat save()',
    messages: [makeMessage({ id: 1, role: 'user', body: 'Still broken' })],
    commits: [
      {
        id: 1,
        ticketId: 'FLG-7X3K9Q',
        commitHash: 'a1b2c3d4e5f6',
        branch: 'fix/crash',
        message: 'fix: guard against nil map',
        createdAt: '2026-07-21T12:00:00Z',
      },
    ],
  });

  it('shows everything about a ticket', async () => {
    const client = stubAdminClient({ getTicket: vi.fn().mockResolvedValue(fullTicket) });
    render(TicketDetail, { props: { client, ticketId: 'FLG-7X3K9Q', lang: 'en' } });

    expect(await screen.findByRole('heading', { name: 'Crash on save' })).toBeInTheDocument();
    expect(screen.getByText('Tapping save closes the app')).toBeInTheDocument();
    expect(screen.getByText('Still broken')).toBeInTheDocument();
    expect(screen.getByText('iPhone 15')).toBeInTheDocument();
    expect(screen.getByText('iOS 18.2')).toBeInTheDocument();
    expect(screen.getByText(/panic: nil map/)).toBeInTheDocument();
    expect(screen.getByText('fix: guard against nil map')).toBeInTheDocument();
    expect(screen.getByText('a1b2c3d4e5')).toBeInTheDocument();
  });

  it('says when there are no logs or commits', async () => {
    const client = stubAdminClient({
      getTicket: vi.fn().mockResolvedValue(makeTicket({ logs: '', commits: [] })),
    });
    render(TicketDetail, { props: { client, ticketId: 'FLG-7X3K9Q', lang: 'en' } });

    expect(await screen.findByText('No logs attached.')).toBeInTheDocument();
    expect(screen.getByText('No commits recorded yet.')).toBeInTheDocument();
  });

  it('fills in missing diagnostics with a dash rather than blank space', async () => {
    const client = stubAdminClient({
      getTicket: vi
        .fn()
        .mockResolvedValue(makeTicket({ appVersion: '', os: '', platform: '', deviceModel: '' })),
    });
    render(TicketDetail, { props: { client, ticketId: 'FLG-7X3K9Q', lang: 'en' } });

    await screen.findByRole('heading', { name: 'Crash on save' });
    expect(screen.getAllByText('—')).toHaveLength(4);
  });

  it('changes status and replies in one step', async () => {
    const client = stubAdminClient({ getTicket: vi.fn().mockResolvedValue(fullTicket) });
    render(TicketDetail, { props: { client, ticketId: 'FLG-7X3K9Q', lang: 'en' } });
    await screen.findByRole('heading', { name: 'Crash on save' });

    await userEvent.selectOptions(screen.getByLabelText('Set status'), 'resolved');
    await userEvent.type(screen.getByLabelText('Reply to reporter'), '  Fixed in main  ');
    await userEvent.click(screen.getByRole('button', { name: 'Save' }));

    await waitFor(() =>
      expect(client.updateTicket).toHaveBeenCalledWith('FLG-7X3K9Q', {
        status: 'resolved',
        shippedVersion: '',
        comment: 'Fixed in main',
      }),
    );
    expect(await screen.findByRole('status')).toHaveTextContent('Ticket updated');
  });

  it('declines a ticket with the reason typed alongside it', async () => {
    const client = stubAdminClient({ getTicket: vi.fn().mockResolvedValue(fullTicket) });
    render(TicketDetail, { props: { client, ticketId: 'FLG-7X3K9Q', lang: 'en' } });
    await screen.findByRole('heading', { name: 'Crash on save' });

    await userEvent.selectOptions(screen.getByLabelText('Set status'), 'declined');
    await userEvent.type(screen.getByLabelText('Reply to reporter'), 'Out of scope for this app.');
    await userEvent.click(screen.getByRole('button', { name: 'Save' }));

    await waitFor(() =>
      expect(client.updateTicket).toHaveBeenCalledWith('FLG-7X3K9Q', {
        status: 'declined',
        shippedVersion: '',
        comment: 'Out of scope for this app.',
      }),
    );
  });

  // A ticket nobody is going to build has no release to name.
  it('asks for no version when declining', async () => {
    const client = stubAdminClient({ getTicket: vi.fn().mockResolvedValue(fullTicket) });
    render(TicketDetail, { props: { client, ticketId: 'FLG-7X3K9Q', lang: 'en' } });
    await screen.findByRole('heading', { name: 'Crash on save' });

    await userEvent.selectOptions(screen.getByLabelText('Set status'), 'declined');

    expect(screen.queryByLabelText('Shipped in version')).not.toBeInTheDocument();
  });

  it('shows a declined ticket as declined', async () => {
    const client = stubAdminClient({
      getTicket: vi.fn().mockResolvedValue(makeTicket({ status: 'declined' })),
    });
    render(TicketDetail, { props: { client, ticketId: 'FLG-7X3K9Q', lang: 'en' } });

    await screen.findByRole('heading', { name: 'Crash on save' });
    expect(screen.getByLabelText('Set status')).toHaveValue('declined');
  });

  it('asks for a version when shipping', async () => {
    const client = stubAdminClient({ getTicket: vi.fn().mockResolvedValue(fullTicket) });
    render(TicketDetail, { props: { client, ticketId: 'FLG-7X3K9Q', lang: 'en' } });
    await screen.findByRole('heading', { name: 'Crash on save' });
    expect(screen.queryByLabelText('Shipped in version')).not.toBeInTheDocument();

    await userEvent.selectOptions(screen.getByLabelText('Set status'), 'shipped');
    await userEvent.type(screen.getByLabelText('Shipped in version'), '1.5.0');
    await userEvent.click(screen.getByRole('button', { name: 'Save' }));

    await waitFor(() =>
      expect(client.updateTicket).toHaveBeenCalledWith(
        'FLG-7X3K9Q',
        expect.objectContaining({ status: 'shipped', shippedVersion: '1.5.0' }),
      ),
    );
  });

  it('reloads after saving so the page matches the server', async () => {
    const client = stubAdminClient({ getTicket: vi.fn().mockResolvedValue(fullTicket) });
    render(TicketDetail, { props: { client, ticketId: 'FLG-7X3K9Q', lang: 'en' } });
    await screen.findByRole('heading', { name: 'Crash on save' });
    expect(client.getTicket).toHaveBeenCalledTimes(1);

    await userEvent.click(screen.getByRole('button', { name: 'Save' }));

    await waitFor(() => expect(client.getTicket).toHaveBeenCalledTimes(2));
  });

  it('shows the version a fix went out in', async () => {
    const client = stubAdminClient({
      getTicket: vi.fn().mockResolvedValue(makeTicket({ status: 'shipped', shippedVersion: '1.5.0' })),
    });
    render(TicketDetail, { props: { client, ticketId: 'FLG-7X3K9Q', lang: 'en' } });

    await screen.findByRole('heading', { name: 'Crash on save' });
    expect(screen.getAllByText('1.5.0').length).toBeGreaterThan(0);
  });

  it('reports a failure to load', async () => {
    const client = stubAdminClient({
      getTicket: vi.fn().mockRejectedValue(apiError('error.notFound', 404)),
    });
    render(TicketDetail, { props: { client, ticketId: 'FLG-ZZZZZZ', lang: 'en' } });

    expect(await screen.findByRole('alert')).toHaveTextContent('No ticket with that ID.');
  });

  it('falls back to a generic load error', async () => {
    const client = stubAdminClient({ getTicket: vi.fn().mockRejectedValue(new Error('kaboom')) });
    render(TicketDetail, { props: { client, ticketId: 'FLG-ZZZZZZ', lang: 'en' } });

    expect(await screen.findByRole('alert')).toHaveTextContent('Something went wrong. Try again.');
  });

  it('reports a failure to save', async () => {
    const client = stubAdminClient({
      getTicket: vi.fn().mockResolvedValue(fullTicket),
      updateTicket: vi.fn().mockRejectedValue(apiError('error.generic', 500)),
    });
    render(TicketDetail, { props: { client, ticketId: 'FLG-7X3K9Q', lang: 'en' } });
    await screen.findByRole('heading', { name: 'Crash on save' });

    await userEvent.click(screen.getByRole('button', { name: 'Save' }));

    expect(await screen.findByRole('alert')).toHaveTextContent('Something went wrong. Try again.');
  });

  it('falls back to a generic save error', async () => {
    const client = stubAdminClient({
      getTicket: vi.fn().mockResolvedValue(fullTicket),
      updateTicket: vi.fn().mockRejectedValue(new Error('kaboom')),
    });
    render(TicketDetail, { props: { client, ticketId: 'FLG-7X3K9Q', lang: 'en' } });
    await screen.findByRole('heading', { name: 'Crash on save' });

    await userEvent.click(screen.getByRole('button', { name: 'Save' }));

    expect(await screen.findByRole('alert')).toHaveTextContent('Something went wrong. Try again.');
  });

  it('goes back to the list', async () => {
    const onback = vi.fn();
    const client = stubAdminClient({ getTicket: vi.fn().mockResolvedValue(fullTicket) });
    render(TicketDetail, { props: { client, ticketId: 'FLG-7X3K9Q', lang: 'en', onback } });

    await userEvent.click(screen.getByRole('button', { name: /All tickets/ }));

    expect(onback).toHaveBeenCalled();
  });

  it('renders in German', async () => {
    const client = stubAdminClient({ getTicket: vi.fn().mockResolvedValue(fullTicket) });
    render(TicketDetail, { props: { client, ticketId: 'FLG-7X3K9Q', lang: 'de' } });

    expect(await screen.findByLabelText('Status setzen')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Speichern' })).toBeInTheDocument();
    expect(screen.getByText('Diagnosedaten')).toBeInTheDocument();
  });
});

describe('Settings', () => {
  it('loads the current configuration', async () => {
    const client = stubAdminClient({
      getSettings: vi
        .fn()
        .mockResolvedValue({ globalAutoProcess: true, hermesWebhookUrl: 'https://hermes.example/hook' }),
    });
    render(Settings, { props: { client, lang: 'en' } });

    expect(await screen.findByLabelText(/Process tickets from new apps/)).toBeChecked();
    expect(screen.getByLabelText('Hermes webhook URL')).toHaveValue('https://hermes.example/hook');
  });

  it('saves the global toggle and the webhook URL', async () => {
    const client = stubAdminClient();
    render(Settings, { props: { client, lang: 'en' } });
    await screen.findByLabelText('Hermes webhook URL');

    await userEvent.click(screen.getByLabelText(/Process tickets from new apps/));
    await userEvent.type(screen.getByLabelText('Hermes webhook URL'), '  https://hermes.example/hook  ');
    await userEvent.click(screen.getByRole('button', { name: 'Save settings' }));

    await waitFor(() =>
      expect(client.updateSettings).toHaveBeenCalledWith({
        globalAutoProcess: true,
        hermesWebhookUrl: 'https://hermes.example/hook',
      }),
    );
    expect(await screen.findByRole('status')).toHaveTextContent('Settings saved');
  });

  it('catches a malformed webhook URL before sending it', async () => {
    const client = stubAdminClient();
    render(Settings, { props: { client, lang: 'en' } });
    await screen.findByLabelText('Hermes webhook URL');

    await userEvent.type(screen.getByLabelText('Hermes webhook URL'), 'hermes.example/hook');
    await userEvent.click(screen.getByRole('button', { name: 'Save settings' }));

    expect(await screen.findByRole('alert')).toHaveTextContent(/must start with http/);
    expect(client.updateSettings).not.toHaveBeenCalled();
  });

  it('accepts an empty webhook URL as "handle it by hand"', async () => {
    const client = stubAdminClient();
    render(Settings, { props: { client, lang: 'en' } });
    await screen.findByLabelText('Hermes webhook URL');

    await userEvent.click(screen.getByRole('button', { name: 'Save settings' }));

    await waitFor(() =>
      expect(client.updateSettings).toHaveBeenCalledWith({
        globalAutoProcess: false,
        hermesWebhookUrl: '',
      }),
    );
  });

  it('lists apps and toggles one', async () => {
    const client = stubAdminClient({
      listApps: vi.fn().mockResolvedValue([makeApp({ name: 'notes', autoProcessEnabled: false })]),
      updateApp: vi.fn().mockResolvedValue(makeApp({ name: 'notes', autoProcessEnabled: true })),
    });
    render(Settings, { props: { client, lang: 'en' } });

    expect(await screen.findByText('notes')).toBeInTheDocument();
    const toggle = screen.getByLabelText('Process automatically');
    expect(toggle).not.toBeChecked();

    await userEvent.click(toggle);

    await waitFor(() => expect(client.updateApp).toHaveBeenCalledWith('notes', true));
    await waitFor(() => expect(screen.getByLabelText('Process automatically')).toBeChecked());
  });

  it('says when no app has reported yet', async () => {
    const client = stubAdminClient({ listApps: vi.fn().mockResolvedValue([]) });
    render(Settings, { props: { client, lang: 'en' } });

    expect(await screen.findByText('No apps yet.')).toBeInTheDocument();
  });

  it('reports a failure to load', async () => {
    const client = stubAdminClient({
      getSettings: vi.fn().mockRejectedValue(apiError('error.network', 0)),
    });
    render(Settings, { props: { client, lang: 'en' } });

    expect(await screen.findByRole('alert')).toHaveTextContent(/Could not reach the server/);
  });

  it('falls back to a generic load error', async () => {
    const client = stubAdminClient({ getSettings: vi.fn().mockRejectedValue(new Error('kaboom')) });
    render(Settings, { props: { client, lang: 'en' } });

    expect(await screen.findByRole('alert')).toHaveTextContent('Something went wrong. Try again.');
  });

  it('reports a failure to save', async () => {
    const client = stubAdminClient({
      updateSettings: vi.fn().mockRejectedValue(apiError('error.generic', 500)),
    });
    render(Settings, { props: { client, lang: 'en' } });
    await screen.findByLabelText('Hermes webhook URL');

    await userEvent.click(screen.getByRole('button', { name: 'Save settings' }));

    expect(await screen.findByRole('alert')).toHaveTextContent('Something went wrong. Try again.');
  });

  it('reports a failure to toggle an app', async () => {
    const client = stubAdminClient({
      updateApp: vi.fn().mockRejectedValue(apiError('error.notFound', 404)),
    });
    render(Settings, { props: { client, lang: 'en' } });
    await screen.findByText('notes');

    await userEvent.click(screen.getByLabelText('Process automatically'));

    expect(await screen.findByRole('alert')).toHaveTextContent('No ticket with that ID.');
  });

  it('falls back to a generic app-toggle error', async () => {
    const client = stubAdminClient({ updateApp: vi.fn().mockRejectedValue(new Error('kaboom')) });
    render(Settings, { props: { client, lang: 'en' } });
    await screen.findByText('notes');

    await userEvent.click(screen.getByLabelText('Process automatically'));

    expect(await screen.findByRole('alert')).toHaveTextContent('Something went wrong. Try again.');
  });

  it('copes with an API that returns nothing', async () => {
    const client = stubAdminClient({
      getSettings: vi.fn().mockResolvedValue(null),
      listApps: vi.fn().mockResolvedValue(null),
    });
    render(Settings, { props: { client, lang: 'en' } });

    expect(await screen.findByText('No apps yet.')).toBeInTheDocument();
    expect(screen.getByLabelText('Hermes webhook URL')).toHaveValue('');
  });

  it('renders in German', async () => {
    render(Settings, { props: { client: stubAdminClient(), lang: 'de' } });

    expect(await screen.findByRole('heading', { name: 'Einstellungen' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Einstellungen speichern' })).toBeInTheDocument();
  });
});

describe('BuildInfo', () => {
  const FULL = '212b000f1e2d3c4b5a69788796a5b4c3d2e1f0aa';

  it('shows the short commit with the full SHA available to inspect', async () => {
    const client = stubAdminClient();
    render(BuildInfo, { props: { client, lang: 'en' } });

    const commit = await screen.findByText('212b000');
    expect(client.getVersion).toHaveBeenCalled();
    expect(commit).toHaveAttribute('title', FULL);
    expect(screen.getByText(/Commit/)).toBeInTheDocument();
  });

  it('copies the full SHA, not the shortened one', async () => {
    const writeText = vi.fn().mockResolvedValue(undefined);
    vi.stubGlobal('navigator', { ...navigator, clipboard: { writeText } });
    render(BuildInfo, { props: { client: stubAdminClient(), lang: 'en' } });
    await screen.findByText('212b000');

    await userEvent.click(screen.getByRole('button', { name: /Copy commit/ }));

    expect(writeText).toHaveBeenCalledWith(FULL);
    expect(await screen.findByText('Copied')).toBeInTheDocument();
    vi.unstubAllGlobals();
  });

  it('stays quiet when the clipboard is blocked', async () => {
    vi.stubGlobal('navigator', {
      ...navigator,
      clipboard: { writeText: vi.fn().mockRejectedValue(new Error('denied')) },
    });
    render(BuildInfo, { props: { client: stubAdminClient(), lang: 'en' } });
    await screen.findByText('212b000');

    await userEvent.click(screen.getByRole('button', { name: /Copy commit/ }));

    expect(screen.queryByText('Copied')).not.toBeInTheDocument();
    expect(screen.getByText('212b000')).toBeInTheDocument();
    vi.unstubAllGlobals();
  });

  it('says unknown when the build carries no commit, and offers nothing to copy', async () => {
    const client = stubAdminClient({
      getVersion: vi.fn().mockResolvedValue({ commit: 'unknown', short: 'unknown', known: false }),
    });
    render(BuildInfo, { props: { client, lang: 'en' } });

    expect(await screen.findByText('unknown')).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /Copy commit/ })).not.toBeInTheDocument();
  });

  it('renders nothing rather than an error when the endpoint fails', async () => {
    const client = stubAdminClient({
      getVersion: vi.fn().mockRejectedValue(apiError('error.generic', 500)),
    });
    const { container } = render(BuildInfo, { props: { client, lang: 'en' } });

    await waitFor(() => expect(client.getVersion).toHaveBeenCalled());
    await waitFor(() => expect(container.querySelector('.build')).toBeNull());
  });

  it('renders in German', async () => {
    render(BuildInfo, { props: { client: stubAdminClient(), lang: 'de' } });

    expect(await screen.findByText(/Commit/)).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /kopieren/i })).toBeInTheDocument();
  });
});

describe('dashboard App', () => {
  // The tickets view has a header of its own, so the `banner` role alone is
  // ambiguous. The shell bar is the one carrying the navigation landmark.
  const shellBar = () => screen.getByRole('navigation').closest('header');

  it('asks for the admin key first', () => {
    render(DashboardApp, { props: { lang: 'en', clientFactory: vi.fn() } });

    expect(screen.getByLabelText('Admin key')).toBeInTheDocument();
    expect(screen.queryByRole('heading', { name: 'Tickets' })).not.toBeInTheDocument();
  });

  it('shows the tickets once signed in', async () => {
    const client = stubAdminClient();
    render(DashboardApp, { props: { lang: 'en', clientFactory: vi.fn().mockReturnValue(client) } });

    await userEvent.type(screen.getByLabelText('Admin key'), 'secret');
    await userEvent.click(screen.getByRole('button', { name: 'Sign in' }));

    expect(await screen.findByRole('heading', { name: 'Tickets' })).toBeInTheDocument();
    expect(sessionStorage.getItem('flagit.adminKey')).toBe('secret');
  });

  it('restores a session from storage', () => {
    sessionStorage.setItem('flagit.adminKey', 'stored-key');
    const clientFactory = vi.fn().mockReturnValue(stubAdminClient());

    render(DashboardApp, { props: { lang: 'en', clientFactory } });

    expect(clientFactory).toHaveBeenCalledWith({ adminKey: 'stored-key' });
    expect(screen.getByRole('heading', { name: 'Tickets' })).toBeInTheDocument();
  });

  it('signs out and forgets the key', async () => {
    render(DashboardApp, { props: { client: stubAdminClient(), lang: 'en' } });
    sessionStorage.setItem('flagit.adminKey', 'secret');

    await userEvent.click(screen.getByRole('button', { name: 'Sign out' }));

    expect(await screen.findByLabelText('Admin key')).toBeInTheDocument();
    expect(sessionStorage.getItem('flagit.adminKey')).toBeNull();
  });

  it('moves between tickets and settings', async () => {
    render(DashboardApp, { props: { client: stubAdminClient(), lang: 'en' } });

    await userEvent.click(screen.getByRole('button', { name: 'Settings' }));
    expect(await screen.findByRole('heading', { name: 'Settings' })).toBeInTheDocument();

    await userEvent.click(screen.getByRole('button', { name: 'Tickets' }));
    expect(await screen.findByRole('heading', { name: 'Tickets' })).toBeInTheDocument();
  });

  it('opens a ticket and comes back', async () => {
    render(DashboardApp, { props: { client: stubAdminClient(), lang: 'en' } });
    await screen.findByText('Crash on save');

    await userEvent.click(screen.getByText('FLG-7X3K9Q'));
    expect(await screen.findByText('Diagnostics')).toBeInTheDocument();

    await userEvent.click(screen.getByRole('button', { name: /All tickets/ }));
    expect(await screen.findByRole('heading', { name: 'Tickets' })).toBeInTheDocument();
  });

  it('opens straight into settings when asked', async () => {
    render(DashboardApp, { props: { client: stubAdminClient(), lang: 'en', view: 'settings' } });

    expect(await screen.findByRole('heading', { name: 'Settings' })).toBeInTheDocument();
  });

  it('switches language across the dashboard', async () => {
    render(DashboardApp, { props: { client: stubAdminClient(), lang: 'en' } });

    await userEvent.click(screen.getByRole('button', { name: /Auf Deutsch umschalten/ }));

    expect(await screen.findByRole('button', { name: 'Abmelden' })).toBeInTheDocument();
    expect(sessionStorage.getItem('flagit.lang')).toBe('de');
  });

  it('detects the language when none is given', () => {
    sessionStorage.setItem('flagit.lang', 'de');

    render(DashboardApp, { props: { client: stubAdminClient() } });

    expect(screen.getByRole('button', { name: 'Abmelden' })).toBeInTheDocument();
  });

  it('keeps the sign-in screen when storage is unavailable', () => {
    const getItem = vi.spyOn(Storage.prototype, 'getItem').mockImplementation(() => {
      throw new Error('blocked');
    });

    render(DashboardApp, { props: { lang: 'en', clientFactory: vi.fn() } });

    expect(screen.getByLabelText('Admin key')).toBeInTheDocument();
    getItem.mockRestore();
  });

  it('signs in even when the key cannot be stored', async () => {
    const setItem = vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => {
      throw new Error('blocked');
    });
    const client = stubAdminClient();

    render(DashboardApp, { props: { lang: 'en', clientFactory: vi.fn().mockReturnValue(client) } });
    await userEvent.type(screen.getByLabelText('Admin key'), 'secret');
    await userEvent.click(screen.getByRole('button', { name: 'Sign in' }));

    expect(await screen.findByRole('heading', { name: 'Tickets' })).toBeInTheDocument();
    setItem.mockRestore();
  });

  it('signs out even when the key cannot be cleared', async () => {
    render(DashboardApp, { props: { client: stubAdminClient(), lang: 'en' } });
    const removeItem = vi.spyOn(Storage.prototype, 'removeItem').mockImplementation(() => {
      throw new Error('blocked');
    });

    await userEvent.click(screen.getByRole('button', { name: 'Sign out' }));

    expect(await screen.findByLabelText('Admin key')).toBeInTheDocument();
    removeItem.mockRestore();
  });

  it('shows the deployed commit once signed in, and never before', async () => {
    const client = stubAdminClient();
    const clientFactory = vi.fn().mockReturnValue(client);
    render(DashboardApp, { props: { lang: 'en', clientFactory } });

    // The sign-in screen has no client, so nothing is fetched and nothing shown.
    expect(screen.queryByText('212b000')).not.toBeInTheDocument();
    expect(client.getVersion).not.toHaveBeenCalled();

    await userEvent.type(screen.getByLabelText('Admin key'), 'secret');
    await userEvent.click(screen.getByRole('button', { name: 'Sign in' }));

    expect(await screen.findByText('212b000')).toBeInTheDocument();
  });

  // The deployed dashboard once rendered this as a page footer below a long
  // ticket list, where nobody ever scrolled to it. It belongs in the header
  // bar, on every view, without scrolling.
  it('shows the commit in the header bar rather than a footer below the fold', async () => {
    const { container } = render(DashboardApp, { props: { client: stubAdminClient(), lang: 'en' } });

    const bar = shellBar();
    expect(bar).toHaveClass('bar');
    expect(await within(bar).findByText('212b000')).toBeInTheDocument();
    expect(within(bar).getByText('Commit')).toBeInTheDocument();
    expect(within(bar).getByRole('button', { name: /Copy commit/ })).toBeInTheDocument();
    expect(container.querySelector('footer')).toBeNull();
  });

  it('keeps the commit visible across views', async () => {
    render(DashboardApp, { props: { client: stubAdminClient(), lang: 'en' } });
    expect(await screen.findByText('212b000')).toBeInTheDocument();

    await userEvent.click(screen.getByRole('button', { name: 'Settings' }));

    expect(await screen.findByRole('heading', { name: 'Settings' })).toBeInTheDocument();
    expect(within(shellBar()).getByText('212b000')).toBeInTheDocument();
  });

  it('marks the current section in the navigation', async () => {
    render(DashboardApp, { props: { client: stubAdminClient(), lang: 'en' } });

    const nav = screen.getByRole('navigation');
    expect(within(nav).getByRole('button', { name: 'Tickets' })).toHaveClass('current');

    await userEvent.click(within(nav).getByRole('button', { name: 'Settings' }));

    expect(within(nav).getByRole('button', { name: 'Settings' })).toHaveClass('current');
  });
});

/*
 * The dashboard driven by the real client over the real response envelopes,
 * rather than by a stub standing in for both.
 *
 * Every other test on this page hands a component a stubbed client, which pins
 * what the UI does with tickets it was given but never that the client can get
 * tickets out of what the server actually sends. The two shapes below are not
 * the same shape, and that asymmetry is the whole reason this exists: the list
 * arrives wrapped twice, as `{"data": {"tickets": [...]}}` with the paging
 * metadata beside it, and the app list arrives wrapped once, as a bare array in
 * `{"data": [...]}`. A client that read `data` and stopped would hand the table
 * a paging object to iterate, and a dashboard reporting nothing is exactly what
 * that looks like from the outside.
 *
 * So the bodies here are transcribed from the deployed internal API rather than
 * built by a helper: a fixture factory that drifts with the client it feeds
 * cannot catch the client and the server disagreeing.
 */
describe('dashboard over the real API envelopes', () => {
  const TICKET = {
    id: 'FLG-B6HNGX',
    type: 'bug',
    title: 'Crash when attaching a screenshot',
    body: 'Picking an image from the library closes the app.',
    status: 'open',
    appName: 'flagit',
    appVersion: '1.4.2',
    os: 'iOS 18.2',
    platform: 'ios',
    deviceModel: 'iPhone 15',
    shippedVersion: '',
    createdAt: '2026-08-10T09:00:00Z',
    updatedAt: '2026-08-10T09:30:00Z',
  };

  /** `GET /internal/tickets` — the page, wrapped in the envelope. */
  const TICKETS_BODY = {
    data: { tickets: [TICKET], total: 1, limit: 100, offset: 0, hasMore: false },
  };

  /** `GET /internal/apps` — a bare array, wrapped in the envelope. */
  const APPS_BODY = {
    data: [{ name: 'flagit', autoProcessEnabled: false, createdAt: '2026-08-01T08:00:00Z' }],
  };

  /** `GET /internal/tickets/{id}` — one flat ticket, conversation included. */
  const DETAIL_BODY = {
    data: {
      ...TICKET,
      messages: [
        {
          id: 1,
          ticketId: 'FLG-B6HNGX',
          body: 'Reproduced on 1.4.2, looking into it.',
          role: 'agent',
          createdAt: '2026-08-10T09:30:00Z',
        },
      ],
      commits: [],
    },
  };

  const VERSION_BODY = { data: { commit: 'f110ac3', short: 'f110ac3', known: true } };

  /**
   * A fetch that answers the internal API's routes and nothing else.
   *
   * Unknown paths get the server's own JSON 404 rather than a thrown error, so
   * a request going somewhere unintended surfaces as the dashboard reporting
   * it — the way it would in a browser — instead of as a stub blowing up.
   */
  function serveInternalApi() {
    const bodies = {
      '/internal/tickets': TICKETS_BODY,
      '/internal/apps': APPS_BODY,
      '/internal/tickets/FLG-B6HNGX': DETAIL_BODY,
      '/internal/version': VERSION_BODY,
      '/internal/settings': { data: { globalAutoProcess: false, hermesWebhookUrl: '' } },
    };
    return vi.fn(async (url) => {
      const path = String(url).split('?')[0];
      const body = bodies[path];
      if (!body) {
        return { ok: false, status: 404, json: async () => ({ error: 'no such endpoint' }) };
      }
      return { ok: true, status: 200, json: async () => body };
    });
  }

  const realClient = (fetchImpl) => createAdminClient({ adminKey: 'admin-key', fetchImpl });

  it('renders the ticket the API sent, through the client that parsed it', async () => {
    const fetchImpl = serveInternalApi();
    render(DashboardApp, { props: { client: realClient(fetchImpl), lang: 'en' } });

    expect(await screen.findByText('FLG-B6HNGX')).toBeInTheDocument();
    expect(screen.getByText('Crash when attaching a screenshot')).toBeInTheDocument();
    expect(screen.queryByText(/No tickets yet/)).not.toBeInTheDocument();
    expect(screen.queryByRole('alert')).toBeNull();
  });

  /*
   * The paging envelope is the one that bites, so this pins the metadata beside
   * the tickets as well: read the wrapper wrong and `tickets` is undefined, and
   * an empty table is indistinguishable from an empty tracker.
   */
  it('reads the page out of the double-wrapped ticket envelope', async () => {
    const fetchImpl = serveInternalApi();

    const page = await realClient(fetchImpl).listTicketPage();

    expect(page.tickets.map((ticket) => ticket.id)).toEqual(['FLG-B6HNGX']);
    expect(page).toMatchObject({ total: 1, limit: 100, offset: 0, hasMore: false });
  });

  it('fills the app filter from the bare-array apps envelope', async () => {
    const fetchImpl = serveInternalApi();
    render(TicketList, { props: { client: realClient(fetchImpl), lang: 'en' } });

    await screen.findByText('FLG-B6HNGX');
    const appFilter = screen.getByLabelText('App');
    expect(within(appFilter).getByRole('option', { name: 'flagit' })).toBeInTheDocument();
  });

  it('opens the ticket detail from the flat ticket envelope', async () => {
    const fetchImpl = serveInternalApi();
    render(TicketDetail, {
      props: { client: realClient(fetchImpl), ticketId: 'FLG-B6HNGX', lang: 'en' },
    });

    expect(
      await screen.findByRole('heading', { name: 'Crash when attaching a screenshot' }),
    ).toBeInTheDocument();
    expect(screen.getByText('Picking an image from the library closes the app.')).toBeInTheDocument();
    expect(screen.getByText('Reproduced on 1.4.2, looking into it.')).toBeInTheDocument();
    expect(screen.getByText('iPhone 15')).toBeInTheDocument();
  });

  /*
   * Where the dashboard asks is as much of the contract as what it asks for.
   * These paths are absolute from the origin root, which is what makes the
   * listener's own root — and a proxy in front of it that rewrites nothing —
   * the arrangement the client is built for. A hop that mounts the dashboard
   * under a prefix instead takes these requests somewhere else, and what comes
   * back is a page rather than an envelope.
   */
  it('asks the internal API at the origin root', async () => {
    const fetchImpl = serveInternalApi();
    render(TicketList, { props: { client: realClient(fetchImpl), lang: 'en' } });
    await screen.findByText('FLG-B6HNGX');

    const paths = fetchImpl.mock.calls.map(([url]) => String(url));
    expect(paths).toContain('/internal/tickets');
    expect(paths).toContain('/internal/apps');
    expect(fetchImpl.mock.calls.every(([, init]) => init.headers['X-Admin-Key'] === 'admin-key')).toBe(
      true,
    );
  });

  /*
   * And the other half of the contract: served a page where an envelope was
   * promised — a misrouted listener, a proxy answering with its own 404 — the
   * dashboard has to say so. It must never quietly report an empty tracker,
   * which is the failure that hid a full database for as long as it did.
   */
  it('reports a page where the envelope should be, instead of an empty tracker', async () => {
    const html = '<!doctype html>\n<html lang="en"><body><div id="app"></div></body></html>';
    const fetchImpl = vi.fn(async () => ({
      ok: true,
      status: 200,
      json: async () => {
        throw new SyntaxError(`Unexpected token < in JSON at position 0: ${html.slice(0, 1)}`);
      },
    }));
    render(TicketList, { props: { client: realClient(fetchImpl), lang: 'en' } });

    expect(await screen.findByRole('alert')).toHaveTextContent(/could not read/i);
    expect(screen.queryByText(/No tickets yet/)).not.toBeInTheDocument();
    expect(screen.queryByText('FLG-B6HNGX')).toBeNull();
  });
});
