<script>
  /**
   * Switch between the two languages. Binary, so it is a single button that
   * names the language you would get — never a dropdown with one other option.
   */
  import { t, otherLanguage } from './i18n.js';

  let { lang = 'en', onchange } = $props();

  const next = $derived(otherLanguage(lang));
</script>

<button type="button" class="toggle mono" onclick={() => onchange?.(next)} title={t('lang.switchTo', lang)}>
  <span class:active={lang === 'en'}>EN</span>
  <span aria-hidden="true" class="sep">/</span>
  <span class:active={lang === 'de'}>DE</span>
  <span class="visually-hidden">{t('lang.switchTo', lang)}</span>
</button>

<style>
  .toggle {
    display: inline-flex;
    gap: 0.35em;
    padding: var(--space-1) var(--space-2);
    color: var(--ink-3);
    background: none;
    border: 1px solid transparent;
    border-radius: var(--radius);
  }

  .toggle:hover {
    border-color: var(--line);
  }

  .active {
    color: var(--ink);
    font-weight: 600;
  }

  .sep {
    opacity: 0.4;
  }
</style>
