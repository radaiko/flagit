<script>
  /**
   * The report form. This is the screen someone reaches mid-frustration, so it
   * asks for the least it can: what kind, one line, some detail. Everything
   * else — version, OS, logs — comes from the host app automatically.
   */
  import { t } from '../lib/i18n.js';
  import FlagTag from '../lib/FlagTag.svelte';

  let {
    client,
    lang = 'en',
    appInfo = {},
    onfiled,
    oncheckstatus,
  } = $props();

  let type = $state('bug');
  let title = $state('');
  let body = $state('');
  let attachDiagnostics = $state(true);
  let submitting = $state(false);
  let errorKey = $state('');
  let filed = $state(null);
  let copied = $state(false);

  const heading = $derived(
    type === 'feature' ? t('create.headingFeature', lang) : t('create.heading', lang),
  );
  const bodyPlaceholder = $derived(
    type === 'feature'
      ? t('create.bodyPlaceholderFeature', lang)
      : t('create.bodyPlaceholderBug', lang),
  );

  async function submit(event) {
    event.preventDefault();
    errorKey = '';

    if (!title.trim()) {
      errorKey = 'error.titleRequired';
      return;
    }
    if (!body.trim()) {
      errorKey = 'error.bodyRequired';
      return;
    }

    submitting = true;
    try {
      filed = await client.createTicket({
        type,
        title: title.trim(),
        body: body.trim(),
        appName: appInfo.appName ?? '',
        appVersion: appInfo.appVersion ?? '',
        os: appInfo.os ?? '',
        platform: appInfo.platform ?? '',
        deviceModel: appInfo.deviceModel ?? '',
        // Diagnostics are opt-out, and opting out must actually send nothing.
        logs: attachDiagnostics ? (appInfo.logs ?? '') : '',
        logsDurationMin: attachDiagnostics ? (appInfo.logsDurationMin ?? 0) : 0,
      });
      onfiled?.(filed);
    } catch (error) {
      errorKey = error.messageKey ?? 'error.generic';
    } finally {
      submitting = false;
    }
  }

  async function copyId() {
    try {
      await navigator.clipboard.writeText(filed.id);
      copied = true;
    } catch {
      // Clipboard access can be denied. The ID is on screen either way, so
      // there is nothing to tell the person that they cannot already see.
    }
  }

  function reset() {
    filed = null;
    copied = false;
    title = '';
    body = '';
    type = 'bug';
  }
</script>

{#if filed}
  <section class="stack done">
    <h1>{t('success.heading', lang)}</h1>
    <FlagTag id={filed.id} status={filed.status} {lang} size="lg" />
    <p class="muted">{t('success.keepId', lang)}</p>
    <div class="actions">
      <button type="button" class="btn" onclick={() => oncheckstatus?.(filed.id)}>
        {t('success.check', lang)}
      </button>
      <button type="button" class="btn btn-secondary" onclick={copyId}>
        {copied ? t('success.copied', lang) : t('success.copy', lang)}
      </button>
    </div>
    <button type="button" class="btn-quiet" onclick={reset}>{t('success.another', lang)}</button>
  </section>
{:else}
  <form class="stack" onsubmit={submit} novalidate>
    <h1>{heading}</h1>

    <fieldset class="kind">
      <legend class="visually-hidden">{t('create.kindLegend', lang)}</legend>
      {#each ['bug', 'feature'] as option}
        <label class="kind-option" class:selected={type === option}>
          <input type="radio" name="type" value={option} bind:group={type} />
          {t(`create.${option}`, lang)}
        </label>
      {/each}
    </fieldset>

    <div class="field">
      <label class="field-label" for="ticket-title">{t('create.titleLabel', lang)}</label>
      <input
        id="ticket-title"
        type="text"
        bind:value={title}
        placeholder={t('create.titlePlaceholder', lang)}
        maxlength="200"
      />
    </div>

    <div class="field">
      <label class="field-label" for="ticket-body">{t('create.bodyLabel', lang)}</label>
      <textarea id="ticket-body" bind:value={body} placeholder={bodyPlaceholder}></textarea>
    </div>

    <label class="diagnostics">
      <input type="checkbox" bind:checked={attachDiagnostics} />
      <span>
        <span class="diagnostics-label">{t('create.diagnosticsLabel', lang)}</span>
        <span class="field-hint">{t('create.diagnosticsHint', lang)}</span>
      </span>
    </label>

    {#if errorKey}
      <p class="alert" role="alert">{t(errorKey, lang)}</p>
    {/if}

    <button type="submit" class="btn btn-block" disabled={submitting}>
      {submitting ? t('create.submitting', lang) : t('create.submit', lang)}
    </button>

    <p class="lookup muted">
      {t('create.lookupPrompt', lang)}
      <button type="button" class="btn-quiet" onclick={() => oncheckstatus?.('')}>
        {t('create.lookupLink', lang)}
      </button>
    </p>
  </form>
{/if}

<style>
  .kind {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 0;
    margin: 0;
    padding: 0;
    border: 1px solid var(--line);
    border-radius: var(--radius);
    overflow: hidden;
  }

  .kind-option {
    padding: var(--space-3);
    text-align: center;
    font-weight: 600;
    color: var(--ink-3);
    cursor: pointer;
  }

  .kind-option + .kind-option {
    border-left: 1px solid var(--line);
  }

  .kind-option.selected {
    color: var(--marker-ink);
    background: var(--marker);
  }

  .kind-option input {
    position: absolute;
    opacity: 0;
    pointer-events: none;
  }

  .kind-option:has(input:focus-visible) {
    outline: var(--focus);
    outline-offset: calc(-1 * var(--focus-offset));
  }

  .diagnostics {
    display: flex;
    gap: var(--space-3);
    align-items: start;
    cursor: pointer;
  }

  .diagnostics input {
    margin-top: 0.3em;
    accent-color: var(--marker);
  }

  .diagnostics span {
    display: block;
  }

  .diagnostics-label {
    font-weight: 600;
  }

  .done {
    align-items: start;
    gap: var(--space-5);
  }

  .actions {
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-3);
  }

  .lookup {
    font-size: var(--step--1);
    text-align: center;
  }
</style>
