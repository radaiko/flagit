<script>
  /**
   * A ticket's conversation. Shared by the overlay and the dashboard so both
   * sides of a thread read identically — the same entry looks the same to the
   * person who filed it and to the developer answering.
   */
  import { t } from './i18n.js';
  import { formatDateTime } from './format.js';

  let { messages = [], lang = 'en', emptyKey = 'view.noMessages' } = $props();
</script>

{#if messages.length === 0}
  <p class="muted empty">{t(emptyKey, lang)}</p>
{:else}
  <ol class="thread">
    {#each messages as message (message.id)}
      <li class="entry" data-role={message.role}>
        <div class="meta mono">
          <span class="author">{message.role === 'user' ? t('view.you', lang) : t('view.team', lang)}</span>
          <time datetime={message.createdAt}>{formatDateTime(message.createdAt, lang)}</time>
        </div>
        <p class="body">{message.body}</p>
      </li>
    {/each}
  </ol>
{/if}

<style>
  .empty {
    font-size: var(--step--1);
  }

  .thread {
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
    margin: 0;
    padding: 0;
    list-style: none;
  }

  .entry {
    padding-left: var(--space-4);
    border-left: 2px solid var(--line);
  }

  /* The agent side is marked, the reporter side is not: in both surfaces the
     reply from the other party is the thing you are scanning for. */
  .entry[data-role='agent'] {
    border-left-color: var(--marker);
  }

  .meta {
    display: flex;
    gap: var(--space-3);
    color: var(--ink-3);
  }

  .author {
    color: var(--ink-2);
  }

  .body {
    margin-top: var(--space-1);
    white-space: pre-wrap;
    overflow-wrap: anywhere;
  }
</style>
