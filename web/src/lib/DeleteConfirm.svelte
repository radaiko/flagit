<script>
  /**
   * The question asked before anything is deleted.
   *
   * Deleting is the only action in the dashboard that cannot be undone, so it
   * is the only one that asks — and it asks the same way everywhere: from a
   * ticket's own screen, from a row in the list, and from a bulk selection.
   * One component rather than three copies, because a warning that is worded
   * or laid out differently in one place is a warning an admin stops reading.
   *
   * It renders nothing until it is opened, and it never calls the API itself:
   * the parent owns the request, the busy flag and the error. All this does is
   * make sure the confirming click is a separate, deliberate one.
   */
  let {
    heading,
    warning,
    confirmLabel,
    cancelLabel,
    busyLabel,
    busy = false,
    error = '',
    onconfirm,
    oncancel,
  } = $props();

  // Unique per instance, so several of these on one screen — a row confirming
  // while the bulk bar is also open — cannot point their headings at each
  // other's text.
  const headingId = $props.id();
</script>

<div class="confirm stack" role="group" aria-labelledby={headingId}>
  <h2 class="eyebrow" id={headingId}>{heading}</h2>
  <p class="warning">{warning}</p>

  {#if error}
    <p class="alert" role="alert">{error}</p>
  {/if}

  <div class="row wrap">
    <button type="button" class="btn btn-danger" onclick={() => onconfirm?.()} disabled={busy}>
      {busy ? busyLabel : confirmLabel}
    </button>
    <button type="button" class="btn btn-secondary" onclick={() => oncancel?.()} disabled={busy}>
      {cancelLabel}
    </button>
  </div>
</div>

<style>
  .confirm {
    padding: var(--space-4);
    background: var(--paper-2);
    border: 1px solid var(--danger);
    border-radius: var(--radius);
  }

  .warning {
    max-width: 52ch;
    color: var(--ink-2);
    font-size: var(--step--1);
  }
</style>
