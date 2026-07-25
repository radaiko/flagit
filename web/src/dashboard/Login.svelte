<script>
  /**
   * Sign in with the admin key. The key is verified by actually calling the
   * API rather than by storing it optimistically, so a wrong key fails here
   * instead of failing later on every screen.
   */
  import { t } from '../lib/i18n.js';
  import { createAdminClient } from '../lib/api.js';

  let { lang = 'en', onauthenticated, clientFactory = createAdminClient } = $props();

  let adminKey = $state('');
  let checking = $state(false);
  let errorKey = $state('');

  async function submit(event) {
    event.preventDefault();
    errorKey = '';

    const key = adminKey.trim();
    if (!key) {
      errorKey = 'admin.keyRequired';
      return;
    }

    checking = true;
    try {
      const client = clientFactory({ adminKey: key });
      await client.getSettings();
      onauthenticated?.(key, client);
    } catch (error) {
      errorKey = error.messageKey ?? 'error.generic';
    } finally {
      checking = false;
    }
  }
</script>

<div class="gate">
  <form class="stack" onsubmit={submit} novalidate>
    <h1>{t('admin.title', lang)}</h1>

    <div class="field">
      <label class="field-label" for="admin-key">{t('admin.keyLabel', lang)}</label>
      <input id="admin-key" type="password" bind:value={adminKey} autocomplete="off" />
      <p class="field-hint">{t('admin.keyHint', lang)}</p>
    </div>

    {#if errorKey}
      <p class="alert" role="alert">{t(errorKey, lang)}</p>
    {/if}

    <button type="submit" class="btn" disabled={checking}>{t('admin.login', lang)}</button>
  </form>
</div>

<style>
  .gate {
    display: flex;
    justify-content: center;
    padding: var(--space-7) var(--space-4);
  }

  form {
    width: 100%;
    max-width: 22rem;
  }

  button {
    align-self: start;
  }
</style>
