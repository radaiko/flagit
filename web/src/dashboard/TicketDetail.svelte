<script>
  /**
   * One ticket, everything about it: what was reported, what the device was,
   * the conversation, the commits an agent produced, and the controls to move
   * it forward. Diagnostics and commits are dev-only and never leave this
   * screen.
   */
  import { t } from '../lib/i18n.js';
  import FlagTag from '../lib/FlagTag.svelte';
  import StatusLabel from '../lib/StatusLabel.svelte';
  import MessageList from '../lib/MessageList.svelte';
  import { formatDate, formatDateTime, STATUSES } from '../lib/format.js';

  let { client, ticketId, lang = 'en', onback } = $props();

  let ticket = $state(null);
  let loading = $state(true);
  let errorKey = $state('');

  let status = $state('open');
  let shippedVersion = $state('');
  let comment = $state('');
  let saving = $state(false);
  let saved = $state(false);

  $effect(() => {
    load(ticketId);
  });

  async function load(id) {
    loading = true;
    errorKey = '';
    try {
      ticket = await client.getTicket(id);
      status = ticket.status;
      shippedVersion = ticket.shippedVersion ?? '';
    } catch (error) {
      errorKey = error.messageKey ?? 'error.generic';
    } finally {
      loading = false;
    }
  }

  async function save() {
    errorKey = '';
    saved = false;
    saving = true;
    try {
      await client.updateTicket(ticket.id, { status, shippedVersion, comment: comment.trim() });
      comment = '';
      saved = true;
      await load(ticket.id);
    } catch (error) {
      errorKey = error.messageKey ?? 'error.generic';
    } finally {
      saving = false;
    }
  }
</script>

<section class="stack">
  <button type="button" class="btn-quiet back" onclick={() => onback?.()}>
    ← {t('detail.back', lang)}
  </button>

  {#if errorKey && !ticket}
    <p class="alert" role="alert">{t(errorKey, lang)}</p>
  {:else if loading && !ticket}
    <p class="muted">{t('list.loading', lang)}</p>
  {:else if ticket}
    <header class="head">
      <FlagTag id={ticket.id} status={ticket.status} {lang} size="lg" />
      <StatusLabel status={ticket.status} {lang} />
    </header>

    <h1>{ticket.title}</h1>

    {#if ticket.body}
      <p class="body">{ticket.body}</p>
    {/if}

    <div class="columns">
      <div class="stack main">
        <h2 class="eyebrow">{t('view.conversation', lang)}</h2>
        <MessageList messages={ticket.messages ?? []} {lang} />

        <div class="controls stack">
          <div class="row wrap">
            <label class="inline">
              <span class="field-label">{t('detail.setStatus', lang)}</span>
              <select bind:value={status}>
                {#each STATUSES as option}
                  <option value={option}>{t(`status.${option}`, lang)}</option>
                {/each}
              </select>
            </label>

            {#if status === 'shipped'}
              <label class="inline">
                <span class="field-label">{t('mass.version', lang)}</span>
                <input
                  type="text"
                  bind:value={shippedVersion}
                  placeholder={t('mass.versionPlaceholder', lang)}
                />
              </label>
            {/if}
          </div>

          <div class="field">
            <label class="field-label" for="admin-reply">{t('detail.replyLabel', lang)}</label>
            <textarea
              id="admin-reply"
              bind:value={comment}
              placeholder={t('detail.replyPlaceholder', lang)}
            ></textarea>
          </div>

          {#if errorKey}
            <p class="alert" role="alert">{t(errorKey, lang)}</p>
          {/if}
          {#if saved}
            <p class="alert alert-ok" role="status">{t('detail.saved', lang)}</p>
          {/if}

          <button type="button" class="btn" onclick={save} disabled={saving}>
            {saving ? t('detail.saving', lang) : t('detail.save', lang)}
          </button>
        </div>
      </div>

      <aside class="stack side">
        <div>
          <h2 class="eyebrow">{t('detail.diagnostics', lang)}</h2>
          <dl class="facts mono">
            <dt>{t('list.colApp', lang)}</dt>
            <dd>{ticket.appName}</dd>
            <dt>{t('view.appVersion', lang)}</dt>
            <dd>{ticket.appVersion || '—'}</dd>
            <dt>{t('detail.os', lang)}</dt>
            <dd>{ticket.os || '—'}</dd>
            <dt>{t('detail.platform', lang)}</dt>
            <dd>{ticket.platform || '—'}</dd>
            <dt>{t('detail.device', lang)}</dt>
            <dd>{ticket.deviceModel || '—'}</dd>
            <dt>{t('view.reportedOn', lang)}</dt>
            <dd>{formatDate(ticket.createdAt, lang)}</dd>
            {#if ticket.shippedVersion}
              <dt>{t('detail.shippedIn', lang)}</dt>
              <dd>{ticket.shippedVersion}</dd>
            {/if}
          </dl>
        </div>

        <div>
          <h2 class="eyebrow">{t('detail.logs', lang)}</h2>
          {#if ticket.logs}
            <pre class="logs">{ticket.logs}</pre>
          {:else}
            <p class="muted small">{t('detail.noLogs', lang)}</p>
          {/if}
        </div>

        <div>
          <h2 class="eyebrow">{t('detail.commits', lang)}</h2>
          {#if (ticket.commits ?? []).length === 0}
            <p class="muted small">{t('detail.noCommits', lang)}</p>
          {:else}
            <ul class="commits">
              {#each ticket.commits as commit (commit.id)}
                <li>
                  <code class="hash">{commit.commitHash.slice(0, 10)}</code>
                  <span class="commit-message">{commit.message}</span>
                  <span class="commit-meta mono">
                    {commit.branch} · {formatDateTime(commit.createdAt, lang)}
                  </span>
                </li>
              {/each}
            </ul>
          {/if}
        </div>
      </aside>
    </div>
  {/if}
</section>

<style>
  .back {
    align-self: start;
  }

  .head {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: var(--space-4);
  }

  .body {
    max-width: 60ch;
    white-space: pre-wrap;
    overflow-wrap: anywhere;
  }

  .columns {
    display: grid;
    grid-template-columns: minmax(0, 2fr) minmax(0, 1fr);
    gap: var(--space-6);
    align-items: start;
  }

  @media (max-width: 60rem) {
    .columns {
      grid-template-columns: 1fr;
    }
  }

  .main {
    min-width: 0;
  }

  .side {
    min-width: 0;
    gap: var(--space-5);
    font-size: var(--step--1);
  }

  .controls {
    padding: var(--space-4);
    background: var(--paper-2);
    border: 1px solid var(--line);
    border-radius: var(--radius);
  }

  .wrap {
    flex-wrap: wrap;
    align-items: end;
  }

  .inline {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
  }

  .inline select,
  .inline input {
    width: auto;
    min-width: 10rem;
  }

  .controls button {
    align-self: start;
  }

  .facts {
    display: grid;
    grid-template-columns: auto 1fr;
    gap: var(--space-2) var(--space-4);
    margin: var(--space-3) 0 0;
  }

  .facts dt {
    color: var(--ink-3);
  }

  .facts dd {
    margin: 0;
    overflow-wrap: anywhere;
  }

  .logs {
    max-height: 20rem;
    margin: var(--space-3) 0 0;
    padding: var(--space-3);
    overflow: auto;
    font-family: var(--font-mono);
    font-size: 0.75rem;
    line-height: 1.5;
    background: var(--paper-2);
    border: 1px solid var(--line);
    border-radius: var(--radius);
  }

  .small {
    margin-top: var(--space-3);
  }

  .commits {
    margin: var(--space-3) 0 0;
    padding: 0;
    list-style: none;
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
  }

  .hash {
    font-family: var(--font-mono);
    color: var(--marker);
  }

  .commit-message,
  .commit-meta {
    display: block;
  }

  .commit-meta {
    color: var(--ink-3);
  }
</style>
