<script>
  import { repositoryLine, rowLabel, summaryFacts } from "./activity.js";

  /** @type {{initials?: string, shortcut?: string, name?: string, repos?: {name: string}[], summary?: {attention?: string, active?: number, coverage?: string, running?: boolean}, unseen?: number, idle?: string, selected?: boolean, [attribute: string]: any}} */
  let {
    initials,
    shortcut = "",
    name,
    repos = [],
    summary = {},
    unseen = 0,
    idle = "",
    selected = false,
    ...rest
  } = $props();

  let repository = $derived(repositoryLine(repos));
  let facts = $derived(summaryFacts(summary, unseen, idle));
  let label = $derived(rowLabel(name ?? "", repos, facts));
  let running = $derived(Boolean(summary.running));
</script>

<!-- Taking focus on the press would leave the keyboard on the row rather than in
     the session it selects, including when that session is already the one shown. -->
<button
  type="button"
  class="item"
  class:selected
  class:running
  aria-current={selected ? "page" : undefined}
  aria-label={label}
  title={label}
  onmousedown={(event) => event.preventDefault()}
  {...rest}>
  <!-- The badge was the loudest thing in the row and was spending it all on a
       keyboard hint, so it carries the session's state as well. -->
  <div class="avatar" class:keyed={shortcut}>
    {shortcut || initials}
  </div>

  <div class="text">
    <div class="name">{name}</div>
    <div class="repositories" title={[repository.name, repository.extra].filter(Boolean).join(" ")}>
      {repository.name}{#if repository.extra}<span class="extra">{repository.extra}</span>{/if}
    </div>
    <div class="facts">
      {#each facts as fact (fact.kind)}
        <span class="fact {fact.kind}">
          {#if fact.kind !== "agents" || fact.active}
            <span class="glyph" class:running={fact.active} aria-hidden="true"
              >{fact.kind === "attention" ? "!" : fact.kind === "unseen" ? "◆" : "●"}</span>
          {/if}
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
    flex: none;
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

  /* Idle is the quietest of the three: no fill, the fainter border, faint text. */
  .avatar {
    position: relative;
    width: 30px;
    height: 30px;
    flex: none;
    display: flex;
    align-items: center;
    justify-content: center;
    font: 700 13px var(--font-machine);
    background: transparent;
    border: 1px solid var(--border-subtle);
    color: var(--text-faint);
  }

  .running .avatar {
    background: var(--surface-raised);
    border-color: var(--border-default);
    color: var(--text-secondary);
  }

  .selected .avatar {
    background: var(--accent-action);
    border-color: var(--accent-action);
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
    color: var(--text-muted);
  }

  .running .name {
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

  .extra {
    margin-left: 0.5ch;
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

  .glyph.running {
    color: var(--state-running);
  }

  .fact.unseen {
    color: var(--state-guided);
  }
</style>
