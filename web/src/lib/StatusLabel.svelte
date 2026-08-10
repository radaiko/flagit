<script>
  /**
   * Status as a word with a colour dot. Deliberately not a filled pill: the
   * flag tag already carries the colour, and two coloured blocks per row would
   * turn a list into confetti. The word is what makes it readable without
   * relying on colour alone.
   */
  import { t } from './i18n.js';

  let { status = 'open', lang = 'en' } = $props();
</script>

<span class="status" data-status={status}>
  <span class="dot" aria-hidden="true"></span>
  {t(`status.${status}`, lang)}
</span>

<style>
  .status {
    --status-color: var(--status-open);
    display: inline-flex;
    align-items: center;
    gap: 0.5em;
    white-space: nowrap;
  }

  .status[data-status='open'] {
    --status-color: var(--status-open);
  }
  .status[data-status='in-progress'] {
    --status-color: var(--status-in-progress);
  }
  .status[data-status='resolved'] {
    --status-color: var(--status-resolved);
  }
  .status[data-status='shipped'] {
    --status-color: var(--status-shipped);
  }
  .status[data-status='closed'] {
    --status-color: var(--status-closed);
  }
  .status[data-status='declined'] {
    --status-color: var(--status-declined);
  }

  .dot {
    width: 0.5em;
    height: 0.5em;
    border-radius: 50%;
    background: var(--status-color);
    flex: none;
  }
</style>
