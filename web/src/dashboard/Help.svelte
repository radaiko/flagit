<script>
  /**
   * The admin's help page. A third top-level view alongside Tickets and
   * Settings, so the answer to "what does shipped actually mean" is one click
   * from the list rather than in a README nobody has open.
   */
  import { t } from '../lib/i18n.js';
  import HelpContent, { ADMIN_TOPICS } from '../lib/HelpContent.svelte';

  let { lang = 'en' } = $props();
</script>

<section class="stack">
  <h1>{t('help.title', lang)}</h1>
  <p class="muted">{t('help.adminIntro', lang)}</p>

  <div class="panel">
    <HelpContent topics={ADMIN_TOPICS} {lang} />
  </div>
</section>

<style>
  .panel {
    max-width: 48rem;
    padding: var(--space-5);
    border: 1px solid var(--line);
    border-radius: var(--radius);
  }
</style>
