/**
 * Deleting, from every place the dashboard offers it.
 *
 * These are rendered tests rather than unit tests on purpose: the thing worth
 * protecting here is not that a function calls an endpoint, it is that nothing
 * is ever deleted without the admin being asked first and answering. That is a
 * property of what is on screen, so it is asserted against what is on screen.
 *
 * Every test drives a stub client. Nothing in this file can reach a real
 * database, which is the point — a test suite that could delete production
 * data is exactly the hazard this feature has to avoid.
 */
import { describe, it, expect, vi } from 'vitest';
import { render, screen, waitFor, within } from '@testing-library/svelte';
import userEvent from '@testing-library/user-event';

import TicketList from '../src/dashboard/TicketList.svelte';
import TicketDetail from '../src/dashboard/TicketDetail.svelte';
import MassOperations from '../src/dashboard/MassOperations.svelte';
import DashboardApp from '../src/dashboard/App.svelte';
import { stubAdminClient, makeTicket, apiError } from './helpers.js';

describe('TicketDetail delete', () => {
  const ticket = makeTicket({ id: 'FLG-7X3K9Q' });

  function renderDetail(overrides = {}, props = {}) {
    const client = stubAdminClient({
      getTicket: vi.fn().mockResolvedValue(ticket),
      ...overrides,
    });
    render(TicketDetail, { props: { client, ticketId: 'FLG-7X3K9Q', lang: 'en', ...props } });
    return client;
  }

  it('offers a clearly labelled delete action', async () => {
    renderDetail();
    await screen.findByRole('heading', { name: 'Crash on save' });

    expect(screen.getByRole('button', { name: 'Delete ticket' })).toBeInTheDocument();
  });

  // The single most important assertion in this file: the button that starts a
  // delete must not perform one.
  it('sends nothing until the deletion is confirmed', async () => {
    const client = renderDetail();
    await screen.findByRole('heading', { name: 'Crash on save' });

    await userEvent.click(screen.getByRole('button', { name: 'Delete ticket' }));

    expect(client.deleteTicket).not.toHaveBeenCalled();
  });

  it('says the deletion is permanent before doing it', async () => {
    renderDetail();
    await screen.findByRole('heading', { name: 'Crash on save' });

    await userEvent.click(screen.getByRole('button', { name: 'Delete ticket' }));

    expect(screen.getByRole('heading', { name: 'Delete this ticket?' })).toBeInTheDocument();
    const warning = screen.getByText(/erases the ticket/i);
    expect(warning).toHaveTextContent('permanent');
    expect(warning).toHaveTextContent('no undo');
  });

  it('deletes the ticket once confirmed and hands back to the list', async () => {
    const ondeleted = vi.fn();
    const client = renderDetail({}, { ondeleted });
    await screen.findByRole('heading', { name: 'Crash on save' });

    await userEvent.click(screen.getByRole('button', { name: 'Delete ticket' }));
    await userEvent.click(screen.getByRole('button', { name: 'Yes, delete permanently' }));

    await waitFor(() => expect(client.deleteTicket).toHaveBeenCalledWith('FLG-7X3K9Q'));
    expect(ondeleted).toHaveBeenCalledWith('FLG-7X3K9Q');
  });

  // With no ondeleted handler the screen still has to leave, because there is
  // nothing left to show.
  it('falls back to onback when no delete handler is given', async () => {
    const onback = vi.fn();
    renderDetail({}, { onback });
    await screen.findByRole('heading', { name: 'Crash on save' });

    await userEvent.click(screen.getByRole('button', { name: 'Delete ticket' }));
    await userEvent.click(screen.getByRole('button', { name: 'Yes, delete permanently' }));

    await waitFor(() => expect(onback).toHaveBeenCalledWith('FLG-7X3K9Q'));
  });

  it('cancels without deleting anything', async () => {
    const ondeleted = vi.fn();
    const client = renderDetail({}, { ondeleted });
    await screen.findByRole('heading', { name: 'Crash on save' });

    await userEvent.click(screen.getByRole('button', { name: 'Delete ticket' }));
    await userEvent.click(screen.getByRole('button', { name: 'Keep the ticket' }));

    expect(client.deleteTicket).not.toHaveBeenCalled();
    expect(ondeleted).not.toHaveBeenCalled();
    // Back to the plain button, so the ticket can still be deleted later.
    expect(screen.getByRole('button', { name: 'Delete ticket' })).toBeInTheDocument();
    expect(
      screen.queryByRole('heading', { name: 'Delete this ticket?' }),
    ).not.toBeInTheDocument();
  });

  // A failed delete leaves the ticket there, so the screen has to stay put and
  // say why rather than pretend the ticket is gone.
  it('keeps the confirmation open and reports a failure', async () => {
    const ondeleted = vi.fn();
    renderDetail(
      { deleteTicket: vi.fn().mockRejectedValue(apiError('error.server', 500)) },
      { ondeleted },
    );
    await screen.findByRole('heading', { name: 'Crash on save' });

    await userEvent.click(screen.getByRole('button', { name: 'Delete ticket' }));
    await userEvent.click(screen.getByRole('button', { name: 'Yes, delete permanently' }));

    expect(await screen.findByRole('alert')).toBeInTheDocument();
    expect(ondeleted).not.toHaveBeenCalled();
    expect(screen.getByRole('heading', { name: 'Delete this ticket?' })).toBeInTheDocument();
  });

  it('asks in German too', async () => {
    renderDetail({}, { lang: 'de' });
    await screen.findByRole('heading', { name: 'Crash on save' });

    await userEvent.click(screen.getByRole('button', { name: 'Ticket löschen' }));

    expect(screen.getByRole('heading', { name: 'Dieses Ticket löschen?' })).toBeInTheDocument();
    expect(
      screen.getByRole('button', { name: 'Ja, endgültig löschen' }),
    ).toBeInTheDocument();
  });
});

describe('TicketList row delete', () => {
  const tickets = [
    makeTicket({ id: 'FLG-AAAAAA', title: 'Alpha' }),
    makeTicket({ id: 'FLG-BBBBBB', title: 'Beta' }),
  ];

  function renderList(overrides = {}, props = {}) {
    const client = stubAdminClient({
      listTickets: vi.fn().mockResolvedValue(tickets),
      ...overrides,
    });
    render(TicketList, { props: { client, lang: 'en', ...props } });
    return client;
  }

  it('offers a delete action on every row', async () => {
    renderList();
    await screen.findByText('Alpha');

    expect(screen.getByRole('button', { name: 'Delete ticket FLG-AAAAAA' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Delete ticket FLG-BBBBBB' })).toBeInTheDocument();
  });

  it('sends nothing until the row deletion is confirmed', async () => {
    const client = renderList();
    await screen.findByText('Alpha');

    await userEvent.click(screen.getByRole('button', { name: 'Delete ticket FLG-AAAAAA' }));

    expect(client.deleteTicket).not.toHaveBeenCalled();
    expect(screen.getByRole('heading', { name: 'Delete this ticket?' })).toBeInTheDocument();
  });

  it('deletes only the row that was asked about, then refreshes the list', async () => {
    const listTickets = vi
      .fn()
      .mockResolvedValueOnce(tickets)
      .mockResolvedValue([tickets[1]]);
    const client = renderList({ listTickets });
    await screen.findByText('Alpha');

    await userEvent.click(screen.getByRole('button', { name: 'Delete ticket FLG-AAAAAA' }));
    await userEvent.click(screen.getByRole('button', { name: 'Yes, delete permanently' }));

    await waitFor(() => expect(client.deleteTicket).toHaveBeenCalledWith('FLG-AAAAAA'));
    expect(client.deleteTicket).toHaveBeenCalledTimes(1);

    // The list reloaded, and the deleted row is gone from it.
    await waitFor(() => expect(screen.queryByText('Alpha')).not.toBeInTheDocument());
    expect(listTickets).toHaveBeenCalledTimes(2);
    expect(screen.getByText('Beta')).toBeInTheDocument();
  });

  it('cancels a row deletion with no request and no change', async () => {
    const client = renderList();
    await screen.findByText('Alpha');

    await userEvent.click(screen.getByRole('button', { name: 'Delete ticket FLG-AAAAAA' }));
    await userEvent.click(screen.getByRole('button', { name: 'Keep the ticket' }));

    expect(client.deleteTicket).not.toHaveBeenCalled();
    expect(client.listTickets).toHaveBeenCalledTimes(1);
    expect(screen.getByText('Alpha')).toBeInTheDocument();
    expect(
      screen.queryByRole('heading', { name: 'Delete this ticket?' }),
    ).not.toBeInTheDocument();
  });

  // Two open questions at once would leave it ambiguous which ticket the
  // confirm button belongs to.
  it('asks about one row at a time', async () => {
    renderList();
    await screen.findByText('Alpha');

    await userEvent.click(screen.getByRole('button', { name: 'Delete ticket FLG-AAAAAA' }));
    await userEvent.click(screen.getByRole('button', { name: 'Delete ticket FLG-BBBBBB' }));

    expect(screen.getAllByRole('heading', { name: 'Delete this ticket?' })).toHaveLength(1);
  });

  it('reports a failed row deletion and keeps the ticket', async () => {
    const client = renderList({
      deleteTicket: vi.fn().mockRejectedValue(apiError('error.server', 500)),
    });
    await screen.findByText('Alpha');

    await userEvent.click(screen.getByRole('button', { name: 'Delete ticket FLG-AAAAAA' }));
    await userEvent.click(screen.getByRole('button', { name: 'Yes, delete permanently' }));

    expect(await screen.findByRole('alert')).toBeInTheDocument();
    expect(screen.getByText('Alpha')).toBeInTheDocument();
    expect(client.listTickets).toHaveBeenCalledTimes(1);
  });
});

describe('MassOperations bulk delete', () => {
  const selected = ['FLG-AAAAAA', 'FLG-BBBBBB'];

  function renderBulk(overrides = {}, props = {}) {
    const client = stubAdminClient(overrides);
    render(MassOperations, { props: { client, selected, lang: 'en', ...props } });
    return client;
  }

  it('offers a clearly labelled bulk delete', () => {
    renderBulk();

    expect(screen.getByRole('button', { name: 'Delete selected' })).toBeInTheDocument();
  });

  it('sends nothing until the bulk deletion is confirmed', async () => {
    const client = renderBulk();

    await userEvent.click(screen.getByRole('button', { name: 'Delete selected' }));

    expect(client.deleteTickets).not.toHaveBeenCalled();
  });

  it('names how many tickets are about to go, and says it is permanent', async () => {
    renderBulk();

    await userEvent.click(screen.getByRole('button', { name: 'Delete selected' }));

    expect(screen.getByRole('heading', { name: 'Delete 2 tickets?' })).toBeInTheDocument();
    const warning = screen.getByText(/erases every selected ticket/i);
    expect(warning).toHaveTextContent('permanent');
    expect(warning).toHaveTextContent('no undo');
  });

  it('deletes the whole selection in one call once confirmed', async () => {
    const ondeleted = vi.fn();
    const client = renderBulk(
      { deleteTickets: vi.fn().mockResolvedValue({ deleted: selected, missing: [] }) },
      { ondeleted },
    );

    await userEvent.click(screen.getByRole('button', { name: 'Delete selected' }));
    await userEvent.click(screen.getByRole('button', { name: 'Yes, delete permanently' }));

    await waitFor(() => expect(client.deleteTickets).toHaveBeenCalledWith(selected));
    expect(client.deleteTickets).toHaveBeenCalledTimes(1);
    expect(ondeleted).toHaveBeenCalledWith({ deleted: selected, missing: [] });
    expect(await screen.findByRole('status')).toHaveTextContent('Deleted 2 tickets');
  });

  it('cancels the bulk deletion with no request', async () => {
    const ondeleted = vi.fn();
    const client = renderBulk({}, { ondeleted });

    await userEvent.click(screen.getByRole('button', { name: 'Delete selected' }));
    await userEvent.click(screen.getByRole('button', { name: 'Keep them' }));

    expect(client.deleteTickets).not.toHaveBeenCalled();
    expect(ondeleted).not.toHaveBeenCalled();
    expect(screen.queryByRole('heading', { name: 'Delete 2 tickets?' })).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Delete selected' })).toBeInTheDocument();
  });

  // IDs that were already gone are reported rather than treated as a failure.
  it('says when some of the selection was already gone', async () => {
    renderBulk({
      deleteTickets: vi
        .fn()
        .mockResolvedValue({ deleted: ['FLG-AAAAAA'], missing: ['FLG-BBBBBB'] }),
    });

    await userEvent.click(screen.getByRole('button', { name: 'Delete selected' }));
    await userEvent.click(screen.getByRole('button', { name: 'Yes, delete permanently' }));

    expect(await screen.findByRole('status')).toHaveTextContent(
      'Deleted 1 tickets, 1 were already gone',
    );
  });

  it('reports a failed bulk deletion without clearing the selection', async () => {
    const ondeleted = vi.fn();
    renderBulk(
      { deleteTickets: vi.fn().mockRejectedValue(apiError('error.server', 500)) },
      { ondeleted },
    );

    await userEvent.click(screen.getByRole('button', { name: 'Delete selected' }));
    await userEvent.click(screen.getByRole('button', { name: 'Yes, delete permanently' }));

    expect(await screen.findByRole('alert')).toBeInTheDocument();
    expect(ondeleted).not.toHaveBeenCalled();
    expect(screen.getByRole('heading', { name: 'Delete 2 tickets?' })).toBeInTheDocument();
  });

  it('asks in the singular for a selection of one', async () => {
    render(MassOperations, {
      props: { client: stubAdminClient(), selected: ['FLG-AAAAAA'], lang: 'en' },
    });

    await userEvent.click(screen.getByRole('button', { name: 'Delete selected' }));

    expect(screen.getByRole('heading', { name: 'Delete this ticket?' })).toBeInTheDocument();
  });

  it('asks in German too', async () => {
    renderBulk({}, { lang: 'de' });

    await userEvent.click(screen.getByRole('button', { name: 'Auswahl löschen' }));

    expect(screen.getByRole('heading', { name: '2 Tickets löschen?' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Ja, endgültig löschen' })).toBeInTheDocument();
  });
});

describe('bulk delete from the list', () => {
  const tickets = [
    makeTicket({ id: 'FLG-AAAAAA', title: 'Alpha' }),
    makeTicket({ id: 'FLG-BBBBBB', title: 'Beta' }),
  ];

  // End to end through the list: select rows, delete them, and the table
  // reloads without them and with the selection dropped.
  it('deletes the selected rows and refreshes the list', async () => {
    const listTickets = vi.fn().mockResolvedValueOnce(tickets).mockResolvedValue([]);
    const client = stubAdminClient({
      listTickets,
      deleteTickets: vi
        .fn()
        .mockResolvedValue({ deleted: ['FLG-AAAAAA', 'FLG-BBBBBB'], missing: [] }),
    });
    render(TicketList, { props: { client, lang: 'en' } });
    await screen.findByText('Alpha');

    await userEvent.click(screen.getByLabelText('Select all'));
    await userEvent.click(screen.getByRole('button', { name: 'Delete selected' }));
    await userEvent.click(screen.getByRole('button', { name: 'Yes, delete permanently' }));

    await waitFor(() =>
      expect(client.deleteTickets).toHaveBeenCalledWith(['FLG-AAAAAA', 'FLG-BBBBBB']),
    );
    await waitFor(() => expect(screen.queryByText('Alpha')).not.toBeInTheDocument());
    expect(listTickets).toHaveBeenCalledTimes(2);
    // The bulk bar only renders while something is selected, so its absence is
    // how the cleared selection shows.
    expect(screen.queryByRole('button', { name: 'Delete selected' })).not.toBeInTheDocument();
  });

  it('does not delete anything when the bulk confirmation is cancelled', async () => {
    const client = stubAdminClient({ listTickets: vi.fn().mockResolvedValue(tickets) });
    render(TicketList, { props: { client, lang: 'en' } });
    await screen.findByText('Alpha');

    await userEvent.click(screen.getByLabelText('Select all'));
    await userEvent.click(screen.getByRole('button', { name: 'Delete selected' }));
    await userEvent.click(screen.getByRole('button', { name: 'Keep them' }));

    expect(client.deleteTickets).not.toHaveBeenCalled();
    expect(client.listTickets).toHaveBeenCalledTimes(1);
    expect(screen.getByText('Alpha')).toBeInTheDocument();
    expect(within(screen.getByRole('table')).getByText('Beta')).toBeInTheDocument();
  });
});

describe('delete through the dashboard shell', () => {
  // TicketDetail falls back to onback when ondeleted is absent, so this passes
  // either way from the outside. It is asserted anyway: the fallback is a
  // courtesy to callers that have nothing better to say, and the shell — which
  // owns the open ticket — is not one of them. Wiring it up here is what makes
  // the detail screen's two exits separable later without a silent regression.
  it('returns to the list when the open ticket is deleted', async () => {
    const survivor = makeTicket({ id: 'FLG-2M8QT4', title: 'Beta' });
    const listTickets = vi
      .fn()
      .mockResolvedValueOnce([makeTicket({ id: 'FLG-7X3K9Q', title: 'Crash on save' }), survivor])
      .mockResolvedValue([survivor]);
    const client = stubAdminClient({ listTickets });
    render(DashboardApp, { props: { client, lang: 'en' } });

    await userEvent.click(await screen.findByText('FLG-7X3K9Q'));
    await screen.findByRole('heading', { name: 'Crash on save' });

    await userEvent.click(screen.getByRole('button', { name: 'Delete ticket' }));
    await userEvent.click(screen.getByRole('button', { name: 'Yes, delete permanently' }));

    await waitFor(() => expect(client.deleteTicket).toHaveBeenCalledWith('FLG-7X3K9Q'));
    // The list is back — and it is the reloaded one, without the ticket that
    // has just gone.
    const table = await screen.findByRole('table');
    expect(within(table).getByText('Beta')).toBeInTheDocument();
    expect(within(table).queryByText('FLG-7X3K9Q')).not.toBeInTheDocument();
    expect(screen.queryByRole('heading', { name: 'Crash on save' })).not.toBeInTheDocument();
  });
});
