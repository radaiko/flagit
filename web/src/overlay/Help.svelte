<script>
  /**
   * The reporter's help screen. Reached from the "?" in the overlay header and
   * leaves the same way it arrived — this is a detour, not a section of the
   * app, so there is exactly one way back.
   */
  import { t } from '../lib/i18n.js';
  import HelpContent, { OVERLAY_TOPICS } from '../lib/HelpContent.svelte';

  let { lang = 'en', onback } = $props();
</script>

<section class="stack">
  <h1>{t('help.title', lang)}</h1>
  <p class="muted">{t('help.overlayIntro', lang)}</p>

  <HelpContent topics={OVERLAY_TOPICS} {lang} />

  <button type="button" class="btn btn-secondary back" onclick={() => onback?.()}>
    {t('help.back', lang)}
  </button>
</section>

<style>
  .back {
    align-self: start;
  }
</style>
