<script>
  /**
   * Dashboard shell: authentication gate, then a two-view app (tickets and
   * settings) with the ticket detail as a third state inside the tickets view.
   *
   * The admin key lives in sessionStorage only — closing the tab signs you out,
   * which is the right default for a key that grants full access to every
   * ticket in the system.
   */
  import { onMount, untrack } from 'svelte';
  import {
    t,
    detectLanguage,
    rememberLanguage,
    applyDocumentLanguage,
  } from '../lib/i18n.js';
  import { createAdminClient, fetchAuthMode } from '../lib/api.js';
  import { stripQueryParam } from '../lib/device.js';
  import LanguageToggle from '../lib/LanguageToggle.svelte';
  import Login from './Login.svelte';
  import TicketList from './TicketList.svelte';
  import TicketDetail from './TicketDetail.svelte';
  import Settings from './Settings.svelte';
  import Help from './Help.svelte';
  import BuildInfo from './BuildInfo.svelte';

  const KEY_STORAGE = 'flagit.adminKey';

  let {
    client: providedClient,
    lang: initialLang,
    view: initialView = 'tickets',
    clientFactory = createAdminClient,
    authMode = fetchAuthMode,
  } = $props();

  // These props seed the initial view; they are not bindings, so they are
  // read once and deliberately not tracked afterwards.
  let lang = $state(untrack(() => initialLang) ?? detectLanguage());
  const initialClient = untrack(() => providedClient) ?? restoreSession();
  let client = $state(initialClient);
  let view = $state(untrack(() => initialView));
  let openTicketId = $state('');

  // A session with no key behind it: the listener itself is the credential.
  // There is nothing to sign out of, so the button is not offered.
  let keyless = $state(false);
  // Only until the probe answers. It is one round trip on a listener we are
  // already talking to, so this renders nothing rather than a spinner that
  // would do little but flash.
  let probing = $state(!initialClient);

  // Some listeners are trusted on their own — the admin dashboard reachable
  // only over the tailnet is — and then there is no key to ask anyone for.
  // The server is the only thing that can say so, and it has to say it
  // explicitly: fetchAuthMode reports "key required" for every other outcome,
  // including a listener that does not answer this route at all.
  onMount(async () => {
    if (!probing) return;
    try {
      const mode = await authMode();
      if (mode?.adminKeyRequired === false) {
        client = clientFactory({ adminKey: '' });
        keyless = true;
      }
    } catch {
      // fetchAuthMode fails closed on its own; a probe passed in as a prop
      // might not, and a rejected probe still means "keep asking for a key".
    } finally {
      probing = false;
    }
  });

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
    keyless = false;
    view = 'tickets';
    openTicketId = '';
  }

  function setLanguage(next) {
    lang = next;
    rememberLanguage(next);
    applyDocumentLanguage(next);
  }

  // Also on mount: the detected language rarely matches the "en" hard-coded in
  // the served HTML.
  $effect(() => {
    applyDocumentLanguage(lang);
  });

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
        <button
          type="button"
          class="nav"
          class:current={view === 'help'}
          onclick={() => show('help')}
        >
          {t('admin.navHelp', lang)}
        </button>
      </nav>

      <div class="row status">
        <BuildInfo {client} {lang} />
        <LanguageToggle {lang} onchange={setLanguage} />
        <button type="button" class="btn-quiet" onclick={signOut}>{t('admin.signOut', lang)}</button>
      </div>
    </header>

    <main>
      {#if view === 'settings'}
        <Settings {client} {lang} />
      {:else if view === 'help'}
        <Help {lang} />
      {:else if openTicketId}
        <!-- ondeleted is passed explicitly even though it does what onback
             does. TicketDetail falls back to onback when it is absent, which
             is right for a caller that has no separate answer, but this shell
             does have one: closing the detail after a delete is not the admin
             navigating back, it is the screen losing the thing it was showing.
             Naming it here keeps that distinction available the moment the two
             need to differ. -->
        <TicketDetail
          {client}
          {lang}
          ticketId={openTicketId}
          onback={() => (openTicketId = '')}
          ondeleted={() => (openTicketId = '')}
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

  /* The build chip, the language toggle and sign-out are one right-hand group.
     The bar wraps rather than overlaps, so on a narrow screen this group drops
     to its own line instead of sitting on top of the navigation. */
  .status {
    flex-wrap: wrap;
    justify-content: flex-end;
    gap: var(--space-3);
  }
</style>
