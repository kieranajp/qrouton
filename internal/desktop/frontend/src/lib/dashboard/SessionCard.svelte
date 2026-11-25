<script>
  import { tick } from "svelte";
  import { age } from "../relative.js";
  import { shortcut } from "../shortcuts.js";
  import { SHOWN_PULLS, repoList, showMore, showMoreLabel, todoItems, todoLine } from "./dashboard.js";
  import PullBlock from "./PullBlock.svelte";

  /** @type {{row: {slug: string, name: string, repos?: {name: string}[], opened: string, todos?: import("./dashboard.js").TodoRollup}, position: number, onOpen: (slug: string) => void, onOpenTodo: (slug: string, path: string, line: number) => void, section: {shown: import("./dashboard.js").PullRequest[], more: number, note: string}, git: string, now: number, onExpand: (slug: string) => void}} */
  let { row, position, onOpen, onOpenTodo, section, git, now, onExpand } = $props();

  let repos = $derived(repoList(row.repos));
  let hint = $derived(shortcut(position));
  let todos = $derived(todoLine(row.todos));
  let items = $derived(todoItems(row.todos));

  /** @type {HTMLElement} */
  let card;

  async function expand() {
    onExpand(row.slug);
    await tick();
    card.querySelectorAll("a")[SHOWN_PULLS]?.focus();
  }
</script>

<article class="card" data-card={row.slug} bind:this={card}>
  <button type="button" class="head" onclick={() => onOpen(row.slug)}>
    <span class="top">
      <span class="title">{row.name}</span>
      {#if hint}<span class="hint">{hint}</span>{/if}
    </span>
    <span class="meta">
      <span class="repos" title={repos}>{repos}</span>
    </span>
    <span class="meta">
      <span class="git">{git}</span>
      <span class="age">{age(row.opened, now)}</span>
    </span>
  </button>
  {#if todos}
    <div class="todos">
      <p class="tally">{todos}</p>
      {#each items as item (`${item.path}:${item.line}`)}
        <button
          type="button"
          class="todo"
          title={item.text}
          onclick={() => onOpenTodo(row.slug, item.path, item.line)}>
          <span class="text">{item.text}</span>
        </button>
      {/each}
    </div>
  {/if}
  {#each section.shown as pr (`${pr.repo}#${pr.number}`)}
    <PullBlock {pr} {now} />
  {/each}
  {#if section.more}
    <button type="button" class="more" aria-label={showMoreLabel(section.more, row.name)} onclick={expand}
      >{showMore(section.more)}</button>
  {/if}
  {#if section.note}
    <p class="note">{section.note}</p>
  {/if}
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
    gap: calc(6px * var(--ui-scale));
    padding: calc(12px * var(--ui-scale)) calc(14px * var(--ui-scale)) calc(11px * var(--ui-scale));
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
    gap: calc(9px * var(--ui-scale));
  }

  .meta {
    align-items: center;
    font: var(--machine-xs);
  }

  .title {
    flex: 1;
    min-width: 0;
    font: var(--display-xs);
    letter-spacing: var(--display-tracking);
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
    padding-top: calc(2px * var(--ui-scale));
    font: var(--machine-xs);
    font-size: calc(10px * var(--ui-scale));
    color: var(--text-faint);
  }

  .repos {
    min-width: 0;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    color: var(--text-secondary);
  }

  .git {
    min-width: 0;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    color: var(--text-muted);
  }

  .age {
    flex: none;
    margin-left: auto;
    color: var(--text-faint);
  }

  .todos {
    display: flex;
    flex-direction: column;
    padding: calc(8px * var(--ui-scale)) 0;
    border-top: 1px solid var(--border-subtle);
  }

  .tally {
    margin: 0;
    padding: 0 calc(14px * var(--ui-scale)) calc(4px * var(--ui-scale));
    font: var(--machine-xs);
    color: var(--text-muted);
  }

  .todo {
    display: flex;
    align-items: baseline;
    gap: calc(8px * var(--ui-scale));
    width: 100%;
    min-width: 0;
    padding: calc(4px * var(--ui-scale)) calc(14px * var(--ui-scale));
    border: 0;
    background: transparent;
    font: var(--machine-xs);
    color: var(--text-secondary);
    text-align: left;
    cursor: pointer;
  }

  .todo:hover {
    background: var(--wash-selected);
    color: var(--text-primary);
  }

  .todo:focus-visible {
    outline: 2px solid var(--accent-action);
    outline-offset: -2px;
  }

  .todo .text {
    flex: 1;
    min-width: 0;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .more {
    width: 100%;
    padding: calc(11px * var(--ui-scale)) calc(14px * var(--ui-scale));
    border: 0;
    border-top: 1px solid var(--border-subtle);
    background: var(--surface-chrome);
    font: var(--machine-xs);
    color: var(--text-secondary);
    text-align: left;
    cursor: pointer;
  }

  .more:hover {
    background: var(--wash-selected);
    color: var(--text-primary);
  }

  .more:focus-visible {
    outline: 2px solid var(--accent-action);
    outline-offset: -2px;
  }

  .note {
    margin: 0;
    padding: calc(11px * var(--ui-scale)) calc(14px * var(--ui-scale));
    border-top: 1px solid var(--border-subtle);
    font: var(--machine-xs);
    color: var(--text-faint);
  }
</style>
