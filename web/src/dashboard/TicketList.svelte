<script>
  /**
   * The triage table. Filters narrow the set, checkboxes build a selection for
   * bulk operations, and clicking a row opens the ticket.
   */
  import { t } from '../lib/i18n.js';
  import FlagTag from '../lib/FlagTag.svelte';
  import StatusLabel from '../lib/StatusLabel.svelte';
  import MassOperations from './MassOperations.svelte';
  import { formatDate, STATUSES, TYPES } from '../lib/format.js';

  let { client, lang = 'en', onopen } = $props();

  let tickets = $state([]);
  let apps = $state([]);
  let loading = $state(true);
  let errorKey = $state('');
  let selected = $state([]);

  let appFilter = $state('');
  let statusFilter = $state('');
  let typeFilter = $state('');

  const filtered = $derived(
    tickets.filter(
      (ticket) =>
        (!appFilter || ticket.appName === appFilter) &&
        (!statusFilter || ticket.status === statusFilter) &&
        (!typeFilter || ticket.type === typeFilter),
    ),
  );

  const allSelected = $derived(filtered.length > 0 && selected.length === filtered.length);

  $effect(() => {
    load();
  });

  async function load() {
    loading = true;
    errorKey = '';
    try {
      const [loadedTickets, loadedApps] = await Promise.all([
        client.listTickets(),
        client.listApps(),
      ]);
      tickets = loadedTickets ?? [];
      apps = loadedApps ?? [];
    } catch (error) {
      errorKey = error.messageKey ?? 'error.generic';
    } finally {
      loading = false;
    }
  }

  function toggle(id) {
    selected = selected.includes(id) ? selected.filter((s) => s !== id) : [...selected, id];
  }

  function toggleAll() {
    selected = allSelected ? [] : filtered.map((ticket) => ticket.id);
  }

  async function afterBatch() {
    selected = [];
    await load();
  }
</script>

<section class="stack">
  <header class="head">
    <h1>{t('list.heading', lang)}</h1>
    <button type="button" class="btn btn-secondary" onclick={load}>{t('list.refresh', lang)}</button>
  </header>

  <div class="filters">
    <label class="inline">
      <span class="field-label">{t('list.filterApp', lang)}</span>
      <select bind:value={appFilter}>
        <option value="">{t('list.filterAll', lang)}</option>
        {#each apps as app (app.name)}
          <option value={app.name}>{app.name}</option>
        {/each}
      </select>
    </label>

    <label class="inline">
      <span class="field-label">{t('list.filterStatus', lang)}</span>
      <select bind:value={statusFilter}>
        <option value="">{t('list.filterAll', lang)}</option>
        {#each STATUSES as status}
          <option value={status}>{t(`status.${status}`, lang)}</option>
        {/each}
      </select>
    </label>

    <label class="inline">
      <span class="field-label">{t('list.filterType', lang)}</span>
      <select bind:value={typeFilter}>
        <option value="">{t('list.filterAll', lang)}</option>
        {#each TYPES as type}
          <option value={type}>{t(`type.${type}`, lang)}</option>
        {/each}
      </select>
    </label>
  </div>

  {#if selected.length > 0}
    <MassOperations
      {client}
      {selected}
      {lang}
      onapplied={afterBatch}
      onclear={() => (selected = [])}
    />
  {/if}

  {#if errorKey}
    <p class="alert" role="alert">{t(errorKey, lang)}</p>
  {:else if loading}
    <p class="muted">{t('list.loading', lang)}</p>
  {:else if filtered.length === 0}
    <p class="muted empty">
      {tickets.length === 0 ? t('list.empty', lang) : t('list.emptyFiltered', lang)}
    </p>
  {:else}
    <table>
      <thead>
        <tr>
          <th class="pick">
            <input
              type="checkbox"
              checked={allSelected}
              onchange={toggleAll}
              aria-label={t('list.selectAll', lang)}
            />
          </th>
          <th class="field-label">{t('list.colId', lang)}</th>
          <th class="field-label">{t('list.colTitle', lang)}</th>
          <th class="field-label">{t('list.colApp', lang)}</th>
          <th class="field-label">{t('list.colStatus', lang)}</th>
          <th class="field-label">{t('list.colCreated', lang)}</th>
        </tr>
      </thead>
      <tbody>
        {#each filtered as ticket (ticket.id)}
          <tr class:selected={selected.includes(ticket.id)}>
            <td class="pick">
              <input
                type="checkbox"
                checked={selected.includes(ticket.id)}
                onchange={() => toggle(ticket.id)}
                aria-label="{t('list.select', lang)} {ticket.id}"
              />
            </td>
            <td>
              <button type="button" class="open" onclick={() => onopen?.(ticket.id)}>
                <FlagTag id={ticket.id} status={ticket.status} {lang} />
              </button>
            </td>
            <td class="title">
              <span class="type mono">{t(`type.${ticket.type}`, lang)}</span>
              {ticket.title}
            </td>
            <td class="mono app">{ticket.appName}</td>
            <td><StatusLabel status={ticket.status} {lang} /></td>
            <td class="mono when">{formatDate(ticket.createdAt, lang)}</td>
          </tr>
        {/each}
      </tbody>
    </table>
  {/if}
</section>

<style>
  .head {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-4);
  }

  .filters {
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-4);
  }

  .inline {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
  }

  .inline select {
    width: auto;
    min-width: 9rem;
  }

  .empty {
    padding: var(--space-6) 0;
  }

  table {
    width: 100%;
    border-collapse: collapse;
    font-size: var(--step--1);
  }

  th {
    text-align: left;
    padding: var(--space-2) var(--space-3);
    color: var(--ink-3);
    border-bottom: 1px solid var(--line);
    font-weight: 400;
  }

  td {
    padding: var(--space-3);
    border-bottom: 1px solid var(--line);
    vertical-align: middle;
  }

  tr.selected td {
    background: var(--paper-2);
  }

  .pick {
    width: 2.5rem;
  }

  .pick input {
    accent-color: var(--marker);
  }

  .open {
    padding: 0;
    background: none;
    border: 0;
  }

  .title {
    font-size: var(--step-0);
    min-width: 16rem;
  }

  .type {
    margin-right: var(--space-2);
    color: var(--ink-3);
  }

  .app,
  .when {
    color: var(--ink-2);
    white-space: nowrap;
  }
</style>
