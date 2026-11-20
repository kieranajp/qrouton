<script>
  import { MODE_LABELS } from "../bridge/generated.js";
  import { age } from "../relative.js";
  import { shortcut } from "../shortcuts.js";
  import { guided, repoList } from "./dashboard.js";

  /** @type {{row: {slug: string, name: string, mode?: string, repos?: {name: string}[], opened: string}, position: number, onOpen: (slug: string) => void}} */
  let { row, position, onOpen } = $props();

  let repos = $derived(repoList(row.repos));
  let hint = $derived(shortcut(position));
</script>

<article class="card" data-card={row.slug}>
  <button type="button" class="head" onclick={() => onOpen(row.slug)}>
    <span class="top">
      <span class="title">{row.name}</span>
      {#if hint}<span class="hint">{hint}</span>{/if}
    </span>
    <span class="meta">
      <span class="repos" title={repos}>{repos}</span>
      {#if guided(row)}<span class="tag">{MODE_LABELS.RPI}</span>{/if}
      <span class="age">{age(row.opened)}</span>
    </span>
  </button>
</article>

<style>
  .card {
    border: 1px solid var(--border-default);
    background: var(--surface-app);
  }

  .head {
    width: 100%;
    box-sizing: border-box;
    display: flex;
    flex-direction: column;
    gap: calc(8px * var(--ui-scale));
    padding: calc(14px * var(--ui-scale));
    border: 0;
    background: transparent;
    color: inherit;
    font: inherit;
    text-align: left;
    cursor: pointer;
  }

  .head:hover {
    background: var(--wash-selected);
  }

  .head:focus-visible {
    outline: 2px solid var(--accent-action);
    outline-offset: -2px;
  }

  .top,
  .meta {
    display: flex;
    gap: calc(8px * var(--ui-scale));
    min-width: 0;
  }

  .top {
    align-items: flex-start;
  }

  .meta {
    align-items: center;
    font: var(--machine-xs);
  }

  .title {
    flex: 1;
    min-width: 0;
    font: var(--display-xs);
    letter-spacing: -0.01em;
    color: var(--text-primary);
    overflow-wrap: anywhere;
    display: -webkit-box;
    -webkit-box-orient: vertical;
    -webkit-line-clamp: 2;
    line-clamp: 2;
    overflow: hidden;
  }

  .hint {
    flex: none;
    font: var(--machine-xs);
    color: var(--text-faint);
  }

  .repos {
    min-width: 0;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    color: var(--text-secondary);
  }

  .tag {
    flex: none;
    padding: 1px calc(4px * var(--ui-scale));
    border: 1px solid currentColor;
    font: 700 calc(9px * var(--ui-scale))/1.2 var(--font-machine);
    letter-spacing: 0.06em;
    text-transform: uppercase;
    color: var(--state-guided);
  }

  .age {
    flex: none;
    margin-left: auto;
    color: var(--text-faint);
  }
</style>
