<script>
  /**
   * Add a message to a ticket you already filed.
   */
  import { t } from '../lib/i18n.js';

  let { client, ticketId, lang = 'en', onsent } = $props();

  let body = $state('');
  let sending = $state(false);
  let errorKey = $state('');
  let sent = $state(false);

  async function submit(event) {
    event.preventDefault();
    errorKey = '';
    sent = false;

    if (!body.trim()) {
      errorKey = 'error.replyRequired';
      return;
    }

    sending = true;
    try {
      const message = await client.postMessage(ticketId, body.trim());
      body = '';
      sent = true;
      onsent?.(message);
    } catch (error) {
      errorKey = error.messageKey ?? 'error.generic';
    } finally {
      sending = false;
    }
  }
</script>

<form class="stack" onsubmit={submit} novalidate>
  <div class="field">
    <label class="field-label" for="reply-body">{t('reply.label', lang)}</label>
    <textarea id="reply-body" bind:value={body} placeholder={t('reply.placeholder', lang)}></textarea>
  </div>

  {#if errorKey}
    <p class="alert" role="alert">{t(errorKey, lang)}</p>
  {/if}
  {#if sent}
    <p class="alert alert-ok" role="status">{t('reply.sent', lang)}</p>
  {/if}

  <button type="submit" class="btn" disabled={sending}>
    {sending ? t('reply.submitting', lang) : t('reply.submit', lang)}
  </button>
</form>

<style>
  form {
    gap: var(--space-3);
  }

  button {
    align-self: start;
  }
</style>
