import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/svelte';
import userEvent from '@testing-library/user-event';

import HelpContent, { OVERLAY_TOPICS, ADMIN_TOPICS } from '../src/lib/HelpContent.svelte';
import OverlayHelp from '../src/overlay/Help.svelte';
import DashboardHelp from '../src/dashboard/Help.svelte';
import OverlayApp from '../src/overlay/App.svelte';
import DashboardApp from '../src/dashboard/App.svelte';
import { translations, LANGUAGES } from '../src/lib/i18n.js';
import { stubPublicClient, stubAdminClient } from './helpers.js';

const ALL_TOPICS = [...OVERLAY_TOPICS, ...ADMIN_TOPICS];

describe('help topics', () => {
  it('has a translation for every key both help screens ask for', () => {
    const keys = ALL_TOPICS.flatMap((topic) => [
      topic.titleKey,
      ...topic.bodyKeys,
      ...(topic.terms ?? []).map((term) => term.descriptionKey),
    ]);

    for (const lang of LANGUAGES) {
      const missing = keys.filter((key) => !(key in translations[lang]));
      expect(missing, `missing ${lang} help text`).toEqual([]);
    }
  });

  it('covers what the reporter and the admin each need', () => {
    const ids = (topics) => topics.map((topic) => topic.id);

    expect(ids(OVERLAY_TOPICS)).toEqual([
      'help.overlay.report',
      'help.overlay.id',
      'help.overlay.replies',
      'help.overlay.device',
      'help.overlay.status',
    ]);
    expect(ids(ADMIN_TOPICS)).toEqual([
      'help.admin.workflow',
      'help.admin.apps',
      'help.admin.mass',
      'help.admin.hermes',
      'help.admin.commits',
      'help.admin.access',
    ]);
  });
});

describe('HelpContent', () => {
  it('renders a heading and every paragraph of a topic', () => {
    render(HelpContent, { props: { topics: [OVERLAY_TOPICS[0]], lang: 'en' } });

    expect(screen.getByRole('heading', { name: 'Filing a report' })).toBeInTheDocument();
    expect(screen.getByText(/Pick Bug if something is broken/)).toBeInTheDocument();
    expect(screen.getByText(/Diagnostics — app version/)).toBeInTheDocument();
  });

  it('lists every status when a topic carries the glossary', () => {
    const statusTopic = OVERLAY_TOPICS.find((topic) => topic.terms);
    render(HelpContent, { props: { topics: [statusTopic], lang: 'en' } });

    for (const status of ['Open', 'In progress', 'Resolved', 'Shipped', 'Closed', 'Declined']) {
      expect(screen.getByText(status)).toBeInTheDocument();
    }
    expect(screen.getByText(/not yet in a release you can install/)).toBeInTheDocument();
  });

  it('renders in German', () => {
    render(HelpContent, { props: { topics: [ADMIN_TOPICS[0]], lang: 'de' } });

    expect(screen.getByRole('heading', { name: 'Der Statusablauf' })).toBeInTheDocument();
    expect(screen.getByText(/Der dokumentierte Weg ist offen/)).toBeInTheDocument();
  });

  it('renders nothing without topics', () => {
    const { container } = render(HelpContent, { props: { lang: 'en' } });

    expect(container.querySelectorAll('section')).toHaveLength(0);
  });
});

describe('overlay Help', () => {
  it('explains the ID, replies and the device token', () => {
    render(OverlayHelp, { props: { lang: 'en' } });

    expect(screen.getByRole('heading', { level: 1, name: 'Help' })).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: 'Your ticket ID' })).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: 'Replies' })).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: 'The device token' })).toBeInTheDocument();
  });

  it('goes back the way it came', async () => {
    let backs = 0;
    render(OverlayHelp, { props: { lang: 'en', onback: () => (backs += 1) } });

    await userEvent.click(screen.getByRole('button', { name: 'Back to reporting' }));

    expect(backs).toBe(1);
  });

  it('survives without a back handler', async () => {
    render(OverlayHelp, { props: { lang: 'en' } });

    await expect(
      userEvent.click(screen.getByRole('button', { name: 'Back to reporting' })),
    ).resolves.not.toThrow();
  });
});

describe('dashboard Help', () => {
  it('explains the workflow, the switches and the integrations', () => {
    render(DashboardHelp, { props: { lang: 'en' } });

    expect(screen.getByRole('heading', { name: 'Status workflow' })).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: 'Per-app settings' })).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: 'Mass operations' })).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: 'Hermes integration' })).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: 'Commit tracking' })).toBeInTheDocument();
  });

  it('renders in German', () => {
    render(DashboardHelp, { props: { lang: 'de' } });

    expect(screen.getByRole('heading', { name: 'Sammelbearbeitung' })).toBeInTheDocument();
  });
});

describe('reaching help', () => {
  it('opens from the overlay header and returns to the screen it left', async () => {
    render(OverlayApp, {
      props: { client: stubPublicClient(), lang: 'en', appInfo: {} },
    });

    await userEvent.click(screen.getByRole('button', { name: 'Help' }));
    expect(screen.getByRole('heading', { level: 1, name: 'Help' })).toBeInTheDocument();

    await userEvent.click(screen.getByRole('button', { name: 'Back to reporting' }));
    expect(screen.getByRole('heading', { name: 'What went wrong?' })).toBeInTheDocument();
  });

  it('returns to the lookup screen when help was opened from there', async () => {
    render(OverlayApp, {
      props: { client: stubPublicClient(), lang: 'en', screen: 'view' },
    });

    await userEvent.click(screen.getByRole('button', { name: 'Help' }));
    await userEvent.click(screen.getByRole('button', { name: 'Back to reporting' }));

    expect(screen.getByRole('heading', { name: 'Look up a ticket' })).toBeInTheDocument();
  });

  it('sits in the dashboard navigation next to Tickets and Settings', async () => {
    render(DashboardApp, { props: { client: stubAdminClient(), lang: 'en' } });

    await userEvent.click(screen.getByRole('button', { name: 'Help' }));

    expect(screen.getByRole('heading', { name: 'Status workflow' })).toBeInTheDocument();
  });
});
