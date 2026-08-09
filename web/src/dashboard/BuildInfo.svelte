<script>
  /**
   * Which commit this Flagit is running, in the dashboard footer.
   *
   * The question it answers is "did my deploy actually land?", so the short SHA
   * is always visible rather than hidden behind a click, and the full SHA is
   * one hover or one copy away — that is the form you paste into `git show`.
   *
   * Build metadata is a nicety, not a feature: if the endpoint fails, or the
   * server predates it, the footer renders nothing at all. An error banner over
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
  <footer class="build">
    <span class="label">{t('admin.commitLabel', lang)}</span>
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
  </footer>
{/if}

<style>
  .build {
    display: flex;
    gap: var(--space-2);
    align-items: baseline;
    padding: var(--space-4) 0;
    margin-top: var(--space-6);
    font-size: var(--step--1);
    color: var(--ink-3);
    border-top: 1px solid var(--line);
  }

  .label {
    text-transform: uppercase;
    letter-spacing: 0.12em;
  }

  .sha {
    color: var(--ink-2);
  }

  .copy {
    padding: 0;
    font-size: inherit;
    color: var(--ink-3);
    background: none;
    border: 0;
    border-bottom: 1px dotted currentColor;
  }
</style>
