<script>
  import { tick } from "svelte";
  import { age } from "../relative.js";
  import { shortcut } from "../shortcuts.js";
  import { SHOWN_PULLS, repoList, showMore, showMoreLabel } from "./dashboard.js";
  import PullBlock from "./PullBlock.svelte";

  /** @type {{row: {slug: string, name: string, repos?: {name: string}[], opened: string}, position: number, onOpen: (slug: string) => void, section: {shown: import("./dashboard.js").PullRequest[], more: number, note: string}, git: string, now: number, onExpand: (slug: string) => void}} */
  let { row, position, onOpen, section, git, now, onExpand } = $props();

  let repos = $derived(repoList(row.repos));
  let hint = $derived(shortcut(position));

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
