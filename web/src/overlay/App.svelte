<script>
  /**
   * The overlay shell: a header, a language toggle, and whichever of the two
   * screens is in play. Sized for a webview panel — one column, nothing that
   * needs a wide viewport.
   */
  import { untrack } from 'svelte';
  import { t, detectLanguage, rememberLanguage } from '../lib/i18n.js';
  import { resolveDeviceToken } from '../lib/device.js';
  import { createPublicClient } from '../lib/api.js';
  import LanguageToggle from '../lib/LanguageToggle.svelte';
  import CreateTicket from './CreateTicket.svelte';
  import ViewTicket from './ViewTicket.svelte';

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
  }

  function showTicket(id) {
    lookupId = id ?? '';
    screen = 'view';
  }

  function showCreate() {
    lookupId = '';
    screen = 'create';
  }
</script>

<div class="overlay">
  <header class="bar">
    <span class="brand mono">{t('app.name', lang)}</span>
    <LanguageToggle {lang} onchange={setLanguage} />
  </header>

  <main>
    {#if screen === 'view'}
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
</style>
