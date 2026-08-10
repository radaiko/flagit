<script>
  /**
   * Bulk status change for the selected tickets. The case this exists for is
   * shipping a release: pick everything that went out, set shipped, name the
   * version once.
   */
  import { t } from '../lib/i18n.js';
  import { STATUSES } from '../lib/format.js';
  import DeleteConfirm from '../lib/DeleteConfirm.svelte';

  let { client, selected = [], lang = 'en', onapplied, ondeleted, onclear } = $props();

  let status = $state('shipped');
  let version = $state('');
  let applying = $state(false);
  let errorKey = $state('');
  let result = $state(null);

  // Deleting keeps its own confirmation, busy flag and error, so a failed
  // status update cannot leave a warning sitting under the delete button and a
  // failed delete cannot be read as a failed update.
  let confirmingDelete = $state(false);
  let deleting = $state(false);
  let deleteErrorKey = $state('');
  let deleteResult = $state(null);

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

  /**
   * Nothing is sent until the confirmation is clicked. The button below only
   * opens the question; this is deliberate, because the selection can be the
   * whole visible table and there is no undo behind it.
   */
  function askToDelete() {
    deleteErrorKey = '';
    deleteResult = null;
    result = null;
    confirmingDelete = true;
  }

  function cancelDelete() {
    deleteErrorKey = '';
    confirmingDelete = false;
  }

  async function confirmDelete() {
    deleteErrorKey = '';
    deleting = true;
    try {
      const outcome = await client.deleteTickets(selected);
      deleteResult = outcome;
      confirmingDelete = false;
      // The rows are gone, so the list has to reload and the selection has to
      // be dropped — both are the parent's job.
      ondeleted?.(outcome);
    } catch (error) {
      // Nothing was deleted; keep the confirmation open so the admin can read
      // what went wrong and decide again.
      deleteErrorKey = error.messageKey ?? 'error.generic';
    } finally {
      deleting = false;
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
  <button type="button" class="btn btn-danger" onclick={askToDelete} disabled={applying}>
    {t('mass.delete', lang)}
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

  {#if confirmingDelete}
    <div class="confirm-slot">
      <DeleteConfirm
        heading={selected.length === 1
          ? t('mass.deleteHeadingOne', lang)
          : t('mass.deleteHeading', lang, { n: selected.length })}
        warning={t('mass.deleteWarning', lang)}
        confirmLabel={t('mass.deleteConfirm', lang)}
        cancelLabel={t('mass.deleteCancel', lang)}
        busyLabel={t('mass.deleting', lang)}
        busy={deleting}
        error={deleteErrorKey ? t(deleteErrorKey, lang) : ''}
        onconfirm={confirmDelete}
        oncancel={cancelDelete}
      />
    </div>
  {/if}

  {#if deleteResult}
    {@const missing = deleteResult.missing?.length ?? 0}
    <p class="alert alert-ok" role="status">
      {missing === 0
        ? t('mass.deleted', lang, { n: deleteResult.deleted.length })
        : t('mass.deletedPartial', lang, { n: deleteResult.deleted.length, missing })}
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

  /* On its own line under the controls, so the question is not squeezed in
     beside the button that raised it. */
  .confirm-slot {
    flex-basis: 100%;
  }
</style>
