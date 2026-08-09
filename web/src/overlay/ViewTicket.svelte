<script>
  /**
   * Look up a ticket by ID and follow it. Two states in one screen: the lookup
   * field, and the ticket once it opens. Keeping them together means someone
   * who mistypes an ID can correct it without navigating anywhere.
   */
  import { untrack } from 'svelte';
  import { t } from '../lib/i18n.js';
  import FlagTag from '../lib/FlagTag.svelte';
  import StatusLabel from '../lib/StatusLabel.svelte';
  import MessageList from '../lib/MessageList.svelte';
  import ReplyForm from './ReplyForm.svelte';
  import { formatDate } from '../lib/format.js';

  let { client, lang = 'en', initialId = '', onback } = $props();

  // Seeded once: the field is the person's to edit after that.
  let ticketId = $state(untrack(() => initialId));
  let ticket = $state(null);
  let loading = $state(false);
  let errorKey = $state('');

  // An ID handed over from the success screen opens straight away, so nobody
  // has to retype the ID they were just given.
  $effect(() => {
    if (initialId && !ticket && !loading && !errorKey) {
      load(initialId);
    }
  });

  async function load(id) {
    errorKey = '';
    // Trimmed, but never re-cased: the server compares ticket IDs byte for
    // byte, so uppercasing here turned every ID that still carries a lowercase
    // character — every ticket filed before the ID alphabet was narrowed — into
    // a 404, including the one the success screen had just handed over.
    const trimmed = (id ?? '').trim();
    if (!trimmed) {
      errorKey = 'error.idRequired';
      return;
    }

    loading = true;
    try {
      ticket = await client.getTicket(trimmed);
      ticketId = trimmed;
    } catch (error) {
      ticket = null;
      errorKey = error.messageKey ?? 'error.generic';
    } finally {
      loading = false;
    }
  }

  function submit(event) {
    event.preventDefault();
    load(ticketId);
  }

  function onReplySent(message) {
    // Append locally rather than refetching: the server already confirmed it,
    // and a round trip would make the reply appear to lag.
    ticket = { ...ticket, messages: [...(ticket.messages ?? []), message] };
  }
</script>

<section class="stack">
  {#if !ticket}
    <h1>{t('view.heading', lang)}</h1>
    <form class="lookup" onsubmit={submit} novalidate>
      <div class="field">
        <label class="field-label" for="lookup-id">{t('view.idLabel', lang)}</label>
        <input
          id="lookup-id"
          type="text"
          class="id-input"
          bind:value={ticketId}
          placeholder={t('view.idPlaceholder', lang)}
          autocapitalize="characters"
          autocomplete="off"
          spellcheck="false"
        />
      </div>
      <button type="submit" class="btn" disabled={loading}>
        {loading ? t('view.loading', lang) : t('view.submit', lang)}
      </button>
    </form>

    {#if errorKey}
      <p class="alert" role="alert">{t(errorKey, lang)}</p>
    {/if}

    <button type="button" class="btn-quiet" onclick={() => onback?.()}>{t('view.back', lang)}</button>
  {:else}
    <header class="head">
      <FlagTag id={ticket.id} status={ticket.status} {lang} />
      <StatusLabel status={ticket.status} {lang} />
    </header>

    <h1>{ticket.title}</h1>

    <dl class="meta mono">
      <div>
        <dt>{t('view.reportedOn', lang)}</dt>
        <dd>{formatDate(ticket.createdAt, lang)}</dd>
      </div>
      <div>
        <dt>{t('view.updatedOn', lang)}</dt>
        <dd>{formatDate(ticket.updatedAt, lang)}</dd>
      </div>
      {#if ticket.shippedVersion}
        <div>
          <dt>{t('detail.shippedIn', lang)}</dt>
          <dd>{ticket.shippedVersion}</dd>
        </div>
      {/if}
    </dl>

    {#if ticket.body}
      <p class="body">{ticket.body}</p>
    {/if}

    <h2 class="eyebrow">{t('view.conversation', lang)}</h2>
    <MessageList messages={ticket.messages ?? []} {lang} />

    <ReplyForm {client} ticketId={ticket.id} {lang} onsent={onReplySent} />

    <button type="button" class="btn-quiet" onclick={() => onback?.()}>{t('view.back', lang)}</button>
  {/if}
</section>

<style>
  .lookup {
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
  }

  .id-input {
    font-family: var(--font-mono);
    font-size: var(--step-1);
    letter-spacing: 0.12em;
    text-transform: uppercase;
  }

  .head {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-3);
    font-size: var(--step--1);
  }

  .meta {
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-5);
    margin: 0;
    color: var(--ink-3);
  }

  .meta dt {
    display: inline;
  }

  .meta dd {
    display: inline;
    margin: 0 0 0 var(--space-2);
    color: var(--ink);
  }

  .body {
    white-space: pre-wrap;
    overflow-wrap: anywhere;
  }
</style>
