<script>
  /**
   * Which commit this Flagit is running, as a status chip in the dashboard bar.
   *
   * The question it answers is "did my deploy actually land?", so the short SHA
   * is always visible rather than hidden behind a click, and the full SHA is
   * one hover or one copy away — that is the form you paste into `git show`.
   *
   * It lives in the header rather than a page footer because the answer is
   * worthless if you have to scroll a long ticket list to reach it: on the
   * deployed dashboard the footer sat below the fold and nobody ever saw it.
   *
   * Build metadata is a nicety, not a feature: if the endpoint fails, or the
   * server predates it, the chip renders nothing at all. An error banner over
   * the dashboard would be a worse answer than silence.
   */
  import { t } from '../lib/i18n.js';

  let { client, lang = 'en' } = $props();

  let info = $state(null);
  let copied = $state(false);

  $effect(() => {
    load();
  });

  async function load() {
    try {
      info = await client.getVersion();
    } catch {
      info = null;
    }
  }

  async function copy() {
    try {
      // The full SHA, never the abbreviation: seven characters are for reading,
      // forty are for pasting into a command.
      await navigator.clipboard.writeText(info.commit);
      copied = true;
    } catch {
      // Clipboard access can be denied. The title attribute still carries the
      // full SHA, so there is nothing to apologise for.
    }
  }
</script>

{#if info}
  <div class="build" data-testid="build-info">
    <span class="label mono">{t('admin.commitLabel', lang)}</span>
    <span class="sha mono" title={info.known ? info.commit : undefined}>
      {info.known ? info.short : t('admin.commitUnknown', lang)}
    </span>
    {#if info.known}
      <button
        type="button"
        class="copy"
        aria-label={t('admin.commitCopy', lang)}
        onclick={copy}
      >
        {copied ? t('admin.commitCopied', lang) : t('admin.commitCopyShort', lang)}
      </button>
    {/if}
  </div>
{/if}

<style>
  .build {
    display: inline-flex;
    gap: var(--space-2);
    align-items: center;
    padding: var(--space-1) var(--space-2);
    font-size: var(--step--1);
    color: var(--ink-3);
    background: var(--paper-2);
    border: 1px solid var(--line);
    border-radius: var(--radius);
    white-space: nowrap;
  }

  .label {
    color: var(--ink-3);
  }

  /* A SHA is read character by character against `git log`, so it is the one
     mono string in the product that keeps its own case. */
  .sha {
    color: var(--ink-2);
    text-transform: none;
    letter-spacing: 0.04em;
  }

  .copy {
    padding: 0;
    font-size: var(--step--1);
    color: var(--ink-3);
    background: none;
    border: 0;
    border-bottom: 1px dotted currentColor;
  }

  .copy:hover {
    color: var(--ink);
  }

  /* On a phone the bar has already wrapped the controls onto their own line;
     drop the word "Commit" so the chip stays a chip and never crowds the nav. */
  @media (max-width: 30rem) {
    .label {
      display: none;
    }
  }
</style>
