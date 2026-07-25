<script>
  /**
   * Bulk status change for the selected tickets. The case this exists for is
   * shipping a release: pick everything that went out, set shipped, name the
   * version once.
   */
  import { t } from '../lib/i18n.js';
  import { STATUSES } from '../lib/format.js';

  let { client, selected = [], lang = 'en', onapplied, onclear } = $props();

  let status = $state('shipped');
  let version = $state('');
  let applying = $state(false);
  let errorKey = $state('');
  let result = $state(null);

  async function apply() {
    errorKey = '';
    result = null;
    applying = true;
    try {
      const outcome = await client.batchUpdate(selected, status, version.trim());
      result = outcome;
      onapplied?.(outcome);
    } catch (error) {
      errorKey = error.messageKey ?? 'error.generic';
    } finally {
      applying = false;
    }
  }
</script>

<aside class="mass" aria-label={t('mass.heading', lang)}>
  <div class="count mono">
    <strong>{selected.length}</strong>
    {t('list.selectedCount', lang)}
  </div>

  <label class="inline">
    <span class="field-label">{t('mass.status', lang)}</span>
    <select bind:value={status}>
      {#each STATUSES as option}
        <option value={option}>{t(`status.${option}`, lang)}</option>
      {/each}
    </select>
  </label>

  {#if status === 'shipped'}
    <label class="inline">
      <span class="field-label">{t('mass.version', lang)}</span>
      <input type="text" bind:value={version} placeholder={t('mass.versionPlaceholder', lang)} />
    </label>
  {/if}

  <button type="button" class="btn" onclick={apply} disabled={applying}>
    {applying ? t('mass.applying', lang) : t('mass.apply', lang)}
  </button>
  <button type="button" class="btn-quiet" onclick={() => onclear?.()}>{t('mass.clear', lang)}</button>

  {#if errorKey}
    <p class="alert" role="alert">{t(errorKey, lang)}</p>
  {/if}
  {#if result}
    {@const failed = Object.keys(result.failed ?? {}).length}
    <p class="alert alert-ok" role="status">
      {failed === 0
        ? t('mass.done', lang, { n: result.updated.length })
        : t('mass.partial', lang, { n: result.updated.length, failed })}
    </p>
  {/if}
</aside>

<style>
  .mass {
    display: flex;
    flex-wrap: wrap;
    align-items: end;
    gap: var(--space-4);
    padding: var(--space-4);
    background: var(--paper-2);
    border: 1px solid var(--line);
    border-radius: var(--radius);
  }

  .count {
    align-self: center;
    color: var(--ink-3);
  }

  .count strong {
    color: var(--ink);
    font-size: var(--step-1);
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

  .alert {
    flex-basis: 100%;
  }
</style>
