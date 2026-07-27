<script>
  /**
   * The overlay shell: a header, a language toggle, and whichever of the two
   * screens is in play. Sized for a webview panel — one column, nothing that
   * needs a wide viewport.
   */
  import { untrack } from 'svelte';
  import {
    t,
    detectLanguage,
    rememberLanguage,
    applyDocumentLanguage,
  } from '../lib/i18n.js';
  import { resolveDeviceToken } from '../lib/device.js';
  import { createPublicClient } from '../lib/api.js';
  import LanguageToggle from '../lib/LanguageToggle.svelte';
  import CreateTicket from './CreateTicket.svelte';
  import ViewTicket from './ViewTicket.svelte';
  import Help from './Help.svelte';

  let {
    client: providedClient,
    lang: initialLang,
    appInfo = {},
    screen: initialScreen = 'create',
  } = $props();

  // Seeded from props once; the overlay owns these after mount.
  let lang = $state(untrack(() => initialLang) ?? detectLanguage());
  let screen = $state(untrack(() => initialScreen));
  let lookupId = $state('');

  // The host app can inject a client (and does, in tests); otherwise build one
  // from the device token in the URL or session.
  const client =
    untrack(() => providedClient) ?? createPublicClient({ deviceToken: resolveDeviceToken() });

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

  function showTicket(id) {
    lookupId = id ?? '';
    screen = 'view';
  }

  function showCreate() {
    lookupId = '';
    screen = 'create';
  }

  // Help is a detour, not a destination: it returns to whatever was on screen
  // when it was opened, so someone reading it mid-report does not lose their
  // place.
  let screenBeforeHelp = $state('create');

  function showHelp() {
    if (screen !== 'help') screenBeforeHelp = screen;
    screen = 'help';
  }

  function closeHelp() {
    screen = screenBeforeHelp;
  }
</script>

<div class="overlay">
  <header class="bar">
    <span class="brand mono">{t('app.name', lang)}</span>
    <div class="row">
      <button
        type="button"
        class="help-link mono"
        aria-current={screen === 'help' ? 'page' : undefined}
        title={t('help.open', lang)}
        onclick={showHelp}
      >
        <span aria-hidden="true">?</span>
        <span class="visually-hidden">{t('help.open', lang)}</span>
      </button>
      <LanguageToggle {lang} onchange={setLanguage} />
    </div>
  </header>

  <main>
    {#if screen === 'help'}
      <Help {lang} onback={closeHelp} />
    {:else if screen === 'view'}
      <ViewTicket {client} {lang} initialId={lookupId} onback={showCreate} />
    {:else}
      <CreateTicket {client} {lang} {appInfo} oncheckstatus={showTicket} />
    {/if}
  </main>
</div>

<style>
  .overlay {
    max-width: 30rem;
    margin: 0 auto;
    padding: var(--space-5) var(--space-4) var(--space-7);
  }

  .bar {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding-bottom: var(--space-4);
    margin-bottom: var(--space-6);
    border-bottom: 1px solid var(--line);
  }

  .brand {
    font-weight: 700;
    letter-spacing: 0.18em;
    color: var(--ink);
  }

  /* Sized to match the language toggle beside it, so the two read as one pair
     of controls rather than a button and an afterthought. */
  .help-link {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 1.75rem;
    height: 1.75rem;
    color: var(--ink-3);
    background: none;
    border: 1px solid var(--line);
    border-radius: 50%;
  }

  .help-link:hover,
  .help-link[aria-current='page'] {
    color: var(--ink);
    border-color: var(--marker);
  }
</style>
