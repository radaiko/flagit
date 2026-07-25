<script>
  /**
   * A ticket ID rendered as a marker tag: a notched plate carrying a stencilled
   * serial, with a colour bar that *is* the status. This is the one device the
   * whole product is built around, so it appears at two scales — large on the
   * screen where someone has to memorise their ID, small in every list row —
   * and nothing else in the UI competes with it.
   */
  import { t } from './i18n.js';

  let { id, status = 'open', lang = 'en', size = 'sm' } = $props();
</script>

<span class="tag {size}" data-status={status}>
  <span class="bar" aria-hidden="true"></span>
  <span class="serial">{id}</span>
  <span class="visually-hidden">— {t('status.label', lang)}: {t(`status.${status}`, lang)}</span>
</span>

<style>
  .tag {
    --status-color: var(--status-open);

    display: inline-flex;
    align-items: stretch;
    gap: 0;
    background: var(--paper-2);
    border: 1px solid var(--line);
    /* The notched corner is what makes this read as a physical tag rather
       than a badge. */
    clip-path: polygon(0.55em 0, 100% 0, 100% 100%, 0.55em 100%, 0 50%);
    padding-left: 0.55em;
  }

  .tag[data-status='open'] {
    --status-color: var(--status-open);
  }
  .tag[data-status='in-progress'] {
    --status-color: var(--status-in-progress);
  }
  .tag[data-status='resolved'] {
    --status-color: var(--status-resolved);
  }
  .tag[data-status='shipped'] {
    --status-color: var(--status-shipped);
  }
  .tag[data-status='closed'] {
    --status-color: var(--status-closed);
  }

  .bar {
    width: 0.4em;
    background: var(--status-color);
  }

  .serial {
    font-family: var(--font-mono);
    font-weight: 600;
    letter-spacing: 0.12em;
    white-space: nowrap;
  }

  .sm {
    font-size: var(--step--1);
  }
  .sm .serial {
    padding: 0.25em 0.75em 0.25em 0.6em;
  }

  .lg {
    font-size: var(--step-2);
  }
  .lg .serial {
    padding: 0.4em 0.7em 0.4em 0.55em;
  }

  /* On the success screen the bar wipes in, then the serial settles. It is the
     only animation in the overlay, and it marks the one moment worth marking. */
  .lg .bar {
    animation: wipe 260ms ease-out both;
  }
  .lg .serial {
    animation: settle 200ms ease-out 180ms both;
  }

  @keyframes wipe {
    from {
      transform: scaleY(0);
    }
    to {
      transform: scaleY(1);
    }
  }

  @keyframes settle {
    from {
      opacity: 0;
      transform: translateX(-0.15em);
    }
    to {
      opacity: 1;
      transform: none;
    }
  }
</style>
