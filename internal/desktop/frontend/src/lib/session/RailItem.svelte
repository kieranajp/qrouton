<script>
  import { repositoryLine, rowLabel, summaryFacts } from "./activity.js";

  /** @type {{initials?: string, shortcut?: string, name?: string, repos?: {name: string}[], summary?: {attention?: string, active?: number, coverage?: string, running?: boolean}, unseen?: number, selected?: boolean, [attribute: string]: any}} */
  let {
    initials,
    shortcut = "",
    name,
    repos = [],
    summary = {},
    unseen = 0,
    selected = false,
    ...rest
  } = $props();

  let repository = $derived(repositoryLine(repos));
  let facts = $derived(summaryFacts(summary, unseen));
  let label = $derived(rowLabel(name ?? "", repos, facts));
</script>

<!-- Taking focus on the press would leave the keyboard on the row rather than in
     the session it selects, including when that session is already the one shown. -->
<button
  type="button"
  class="item"
  class:selected
  aria-current={selected ? "page" : undefined}
  aria-label={label}
  title={label}
  onmousedown={(event) => event.preventDefault()}
  {...rest}>
  <div class="avatar" class:keyed={shortcut}>
    {shortcut || initials}
  </div>

  <div class="text">
    <div class="name">{name}</div>
    <div class="repositories" title={repository}>{repository}</div>
    <div class="facts">
      {#each facts as fact (fact.kind)}
        <span class="fact {fact.kind}">
          <span class="glyph" aria-hidden="true">{fact.kind === "attention" ? "!" : fact.kind === "unseen" ? "◆" : "●"}</span>
          {fact.label}
        </span>
      {/each}
    </div>
  </div>
</button>

<style>
  .item {
    display: flex;
    align-items: center;
    gap: 10px;
    width: 100%;
    padding: 8px 9px;
    cursor: pointer;
    border: 1px solid transparent;
    background: transparent;
    font: inherit;
    color: inherit;
    text-align: left;
  }

  .item:hover:not(.selected) {
    border-color: var(--border-default);
    background: var(--surface-raised);
  }

  .item:focus-visible {
    outline: 2px solid var(--accent-action);
    outline-offset: -2px;
  }

  .item:hover .name {
    color: var(--text-primary);
  }

  .selected {
    border-color: var(--accent-action);
    background: var(--surface-raised);
    box-shadow: var(--shadow-focus);
  }

  .avatar {
    position: relative;
    width: 30px;
    height: 30px;
    flex: none;
    display: flex;
    align-items: center;
    justify-content: center;
    font: 700 13px var(--font-machine);
    background: var(--surface-raised);
    border: 1px solid var(--border-default);
    color: var(--text-secondary);
  }

  .selected .avatar {
    background: var(--accent-action);
    border: none;
    color: var(--text-on-accent);
  }

  /* A shortcut is punctuation, not a word: the initials' weight makes it shout. */
  .keyed {
    font: var(--machine-md);
    font-size: 11px;
  }

  .text {
    min-width: 0;
    flex: 1;
  }

  .name,
  .repositories {
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .name {
    font: var(--machine-md);
    font-size: 11px;
    color: var(--text-secondary);
  }

  .selected .name {
    font: var(--machine-bold);
    font-size: 11px;
    color: var(--text-primary);
  }

  .repositories,
  .facts {
    font: var(--machine-xs);
    font-size: 9.5px;
    margin-top: 2px;
    color: var(--text-faint);
  }

  .selected .repositories,
  .selected .facts {
    color: var(--text-muted);
  }

  .facts {
    display: flex;
    flex-wrap: wrap;
    column-gap: 6px;
    row-gap: 1px;
  }

  .fact {
    flex: none;
  }

  .fact.attention {
    color: var(--state-waiting);
  }

  .fact.unseen {
    color: var(--state-guided);
  }
</style>
