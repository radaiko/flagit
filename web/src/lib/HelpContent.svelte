<script module>
  /**
   * The help text, as data.
   *
   * Both help screens render the same component; only the topic list differs.
   * Keeping the lists here rather than in each screen means the reporter's help
   * and the admin's help cannot drift apart in structure, and a test can walk
   * every key that either of them will ask for.
   */

  // Every status, in the one order the backend documents — imported rather
  // than restated, so the glossary cannot fall behind the workflow.
  import { STATUSES } from './format.js';

  /** The status glossary, shown wherever the workflow is being explained. */
  const STATUS_TERMS = STATUSES.map((status) => ({
    status,
    descriptionKey: `help.status.${status}`,
  }));

  /**
   * Describe one topic. Bodies are numbered `body1`, `body2`, … under the
   * topic's own prefix, so adding a paragraph is a translation change and not
   * a code change.
   */
  function topic(prefix, { paragraphs = 2, terms = null } = {}) {
    return {
      id: prefix,
      titleKey: `${prefix}.title`,
      bodyKeys: Array.from({ length: paragraphs }, (_, i) => `${prefix}.body${i + 1}`),
      terms,
    };
  }

  /** What someone filing a report needs to know. */
  export const OVERLAY_TOPICS = [
    topic('help.overlay.report'),
    topic('help.overlay.id'),
    topic('help.overlay.replies'),
    topic('help.overlay.device'),
    topic('help.overlay.status', { paragraphs: 1, terms: STATUS_TERMS }),
  ];

  /** What someone running Flagit needs to know. */
  export const ADMIN_TOPICS = [
    topic('help.admin.workflow', { terms: STATUS_TERMS }),
    topic('help.admin.apps'),
    topic('help.admin.mass'),
    topic('help.admin.hermes'),
    topic('help.admin.commits'),
    topic('help.admin.access'),
  ];
</script>

<script>
  import { t } from './i18n.js';
  import StatusLabel from './StatusLabel.svelte';

  let { topics = [], lang = 'en' } = $props();
</script>

<div class="help">
  {#each topics as item (item.id)}
    <section class="topic">
      <h2>{t(item.titleKey, lang)}</h2>
      {#each item.bodyKeys as key (key)}
        <p>{t(key, lang)}</p>
      {/each}

      {#if item.terms}
        <dl class="terms">
          {#each item.terms as term (term.status)}
            <div>
              <dt><StatusLabel status={term.status} {lang} /></dt>
              <dd>{t(term.descriptionKey, lang)}</dd>
            </div>
          {/each}
        </dl>
      {/if}
    </section>
  {/each}
</div>

<style>
  .help {
    display: flex;
    flex-direction: column;
    gap: var(--space-6);
    max-width: 44rem;
  }

  .topic {
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
  }

  /* A rule under the heading, the way a marked-up plan separates sections.
     Cheaper to scan than a box around every topic. */
  .topic h2 {
    padding-bottom: var(--space-2);
    font-size: var(--step-1);
    border-bottom: 1px solid var(--line);
  }

  .topic p {
    color: var(--ink-2);
  }

  .terms {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
    margin: var(--space-1) 0 0;
  }

  .terms div {
    display: grid;
    grid-template-columns: minmax(7rem, max-content) 1fr;
    gap: var(--space-2) var(--space-4);
    align-items: baseline;
  }

  .terms dt {
    font-size: var(--step--1);
    font-weight: 600;
  }

  .terms dd {
    margin: 0;
    color: var(--ink-2);
  }

  /* Two columns stop being two columns on a phone-width overlay. */
  @media (max-width: 30rem) {
    .terms div {
      grid-template-columns: 1fr;
      gap: 0;
    }
  }
</style>
