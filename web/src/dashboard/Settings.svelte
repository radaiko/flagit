<script>
  /**
   * Global configuration and per-app automation.
   *
   * The two switches interact, so the copy says which one wins: the global
   * toggle only decides what happens the first time Flagit sees an app; after
   * that, the app's own switch is the answer.
   */
  import { t } from '../lib/i18n.js';
  import { formatDate } from '../lib/format.js';

  let { client, lang = 'en' } = $props();

  let settings = $state(null);
  let apps = $state([]);
  let loading = $state(true);
  let saving = $state(false);
  let saved = $state(false);
  let errorKey = $state('');

  let globalAutoProcess = $state(false);
  let webhookUrl = $state('');

  $effect(() => {
    load();
  });

  async function load() {
    loading = true;
    errorKey = '';
    try {
      const [loadedSettings, loadedApps] = await Promise.all([
        client.getSettings(),
        client.listApps(),
      ]);
      settings = loadedSettings;
      apps = loadedApps ?? [];
      globalAutoProcess = loadedSettings?.globalAutoProcess ?? false;
      webhookUrl = loadedSettings?.hermesWebhookUrl ?? '';
    } catch (error) {
      errorKey = error.messageKey ?? 'error.generic';
    } finally {
      loading = false;
    }
  }

  async function save() {
    errorKey = '';
    saved = false;

    const url = webhookUrl.trim();
    // Checked here as well as on the server so the person finds out while
    // their cursor is still in the field.
    if (url && !/^https?:\/\//.test(url)) {
      errorKey = 'settings.webhookInvalid';
      return;
    }

    saving = true;
    try {
      settings = await client.updateSettings({
        globalAutoProcess,
        hermesWebhookUrl: url,
      });
      saved = true;
    } catch (error) {
      errorKey = error.messageKey ?? 'error.generic';
    } finally {
      saving = false;
    }
  }

  async function toggleApp(app) {
    errorKey = '';
    try {
      const updated = await client.updateApp(app.name, !app.autoProcessEnabled);
      apps = apps.map((candidate) => (candidate.name === app.name ? updated : candidate));
    } catch (error) {
      errorKey = error.messageKey ?? 'error.generic';
    }
  }
</script>

<section class="stack">
  <h1>{t('settings.heading', lang)}</h1>

  {#if loading}
    <p class="muted">{t('list.loading', lang)}</p>
  {:else}
    <div class="panel stack">
      <h2 class="eyebrow">{t('settings.autoHeading', lang)}</h2>

      <label class="switch">
        <input type="checkbox" bind:checked={globalAutoProcess} />
        <span>
          <span class="switch-label">{t('settings.globalAuto', lang)}</span>
          <span class="field-hint">{t('settings.globalAutoHint', lang)}</span>
        </span>
      </label>

      <div class="field">
        <label class="field-label" for="webhook-url">{t('settings.webhookLabel', lang)}</label>
        <input id="webhook-url" type="text" bind:value={webhookUrl} placeholder="https://" />
        <p class="field-hint">{t('settings.webhookHint', lang)}</p>
      </div>

      {#if errorKey}
        <p class="alert" role="alert">{t(errorKey, lang)}</p>
      {/if}
      {#if saved}
        <p class="alert alert-ok" role="status">{t('settings.saved', lang)}</p>
      {/if}

      <button type="button" class="btn" onclick={save} disabled={saving}>
        {saving ? t('settings.saving', lang) : t('settings.save', lang)}
      </button>
    </div>

    <div class="panel stack">
      <h2 class="eyebrow">{t('settings.appsHeading', lang)}</h2>
      <p class="field-hint">{t('settings.appsHint', lang)}</p>

      {#if apps.length === 0}
        <p class="muted">{t('settings.appsEmpty', lang)}</p>
      {:else}
        <ul class="apps">
          {#each apps as app (app.name)}
            <li>
              <div>
                <span class="app-name mono">{app.name}</span>
                <span class="field-hint">
                  {t('settings.appSince', lang)}
                  {formatDate(app.createdAt, lang)}
                </span>
              </div>
              <label class="switch compact">
                <input
                  type="checkbox"
                  checked={app.autoProcessEnabled}
                  onchange={() => toggleApp(app)}
                />
                <span class="switch-label">{t('settings.appAuto', lang)}</span>
              </label>
            </li>
          {/each}
        </ul>
      {/if}
    </div>
  {/if}
</section>

<style>
  .panel {
    max-width: 44rem;
    padding: var(--space-5);
    border: 1px solid var(--line);
    border-radius: var(--radius);
  }

  .switch {
    display: flex;
    gap: var(--space-3);
    align-items: start;
    cursor: pointer;
  }

  .switch input {
    margin-top: 0.3em;
    accent-color: var(--marker);
  }

  .switch span {
    display: block;
  }

  .switch-label {
    font-weight: 600;
  }

  .compact {
    align-items: center;
    font-size: var(--step--1);
    white-space: nowrap;
  }

  .compact input {
    margin-top: 0;
  }

  .apps {
    margin: 0;
    padding: 0;
    list-style: none;
  }

  .apps li {
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-4);
    align-items: center;
    justify-content: space-between;
    padding: var(--space-3) 0;
    border-top: 1px solid var(--line);
  }

  .app-name {
    display: block;
    font-size: var(--step-0);
    font-weight: 600;
    color: var(--ink);
    text-transform: none;
    letter-spacing: 0.04em;
  }

  button {
    align-self: start;
  }
</style>
