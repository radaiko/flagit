<script>
  /**
   * Dashboard shell: authentication gate, then a two-view app (tickets and
   * settings) with the ticket detail as a third state inside the tickets view.
   *
   * The admin key lives in sessionStorage only — closing the tab signs you out,
   * which is the right default for a key that grants full access to every
   * ticket in the system.
   */
  import { untrack } from 'svelte';
  import { t, detectLanguage, rememberLanguage } from '../lib/i18n.js';
  import { createAdminClient } from '../lib/api.js';
  import { stripQueryParam } from '../lib/device.js';
  import LanguageToggle from '../lib/LanguageToggle.svelte';
  import Login from './Login.svelte';
  import TicketList from './TicketList.svelte';
  import TicketDetail from './TicketDetail.svelte';
  import Settings from './Settings.svelte';

  const KEY_STORAGE = 'flagit.adminKey';

  let {
    client: providedClient,
    lang: initialLang,
    view: initialView = 'tickets',
    clientFactory = createAdminClient,
  } = $props();

  // These props seed the initial view; they are not bindings, so they are
  // read once and deliberately not tracked afterwards.
  let lang = $state(untrack(() => initialLang) ?? detectLanguage());
  let client = $state(untrack(() => providedClient) ?? restoreSession());
  let view = $state(untrack(() => initialView));
  let openTicketId = $state('');

  function restoreSession() {
    const key = readStoredKey();
    return key ? clientFactory({ adminKey: key }) : null;
  }

  function readStoredKey() {
    // An admin arriving from a link can pass the key once. It moves into
    // session storage and is stripped from the URL immediately: a key left in
    // the address bar lands in browser history and in the Referer header of
    // any outbound link, and it grants access to every ticket in the system.
    try {
      const fromUrl = new URLSearchParams(globalThis.location?.search ?? '').get('admin');
      if (fromUrl) {
        sessionStorage.setItem(KEY_STORAGE, fromUrl);
        stripQueryParam('admin');
        return fromUrl;
      }
      return sessionStorage.getItem(KEY_STORAGE);
    } catch {
      stripQueryParam('admin');
      return null;
    }
  }

  function onAuthenticated(key, authenticatedClient) {
    try {
      sessionStorage.setItem(KEY_STORAGE, key);
    } catch {
      // Storage can be unavailable; the session still works until reload.
    }
    client = authenticatedClient;
  }

  function signOut() {
    try {
      sessionStorage.removeItem(KEY_STORAGE);
    } catch {
      // Nothing to clear.
    }
    client = null;
    view = 'tickets';
    openTicketId = '';
  }

  function setLanguage(next) {
    lang = next;
    rememberLanguage(next);
  }

  function show(nextView) {
    view = nextView;
    openTicketId = '';
  }
</script>

{#if !client}
  <Login {lang} {clientFactory} onauthenticated={onAuthenticated} />
{:else}
  <div class="shell">
    <header class="bar">
      <span class="brand mono">{t('app.name', lang)}</span>

      <nav>
        <button
          type="button"
          class="nav"
          class:current={view === 'tickets'}
          onclick={() => show('tickets')}
        >
          {t('admin.navTickets', lang)}
        </button>
        <button
          type="button"
          class="nav"
          class:current={view === 'settings'}
          onclick={() => show('settings')}
        >
          {t('admin.navSettings', lang)}
        </button>
      </nav>

      <div class="row">
        <LanguageToggle {lang} onchange={setLanguage} />
        <button type="button" class="btn-quiet" onclick={signOut}>{t('admin.signOut', lang)}</button>
      </div>
    </header>

    <main>
      {#if view === 'settings'}
        <Settings {client} {lang} />
      {:else if openTicketId}
        <TicketDetail
          {client}
          {lang}
          ticketId={openTicketId}
          onback={() => (openTicketId = '')}
        />
      {:else}
        <TicketList {client} {lang} onopen={(id) => (openTicketId = id)} />
      {/if}
    </main>
  </div>
{/if}

<style>
  .shell {
    max-width: 78rem;
    margin: 0 auto;
    padding: 0 var(--space-5) var(--space-7);
  }

  .bar {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: var(--space-5);
    padding: var(--space-4) 0;
    margin-bottom: var(--space-6);
    border-bottom: 1px solid var(--line);
  }

  .brand {
    font-weight: 700;
    letter-spacing: 0.18em;
  }

  nav {
    display: flex;
    gap: var(--space-1);
    margin-right: auto;
  }

  .nav {
    padding: var(--space-2) var(--space-3);
    color: var(--ink-3);
    background: none;
    border: 0;
    border-bottom: 2px solid transparent;
  }

  .nav.current {
    color: var(--ink);
    font-weight: 600;
    border-bottom-color: var(--marker);
  }
</style>
