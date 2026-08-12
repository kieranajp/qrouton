<script>
  import StatusDot from "../core/StatusDot.svelte";

  /** @returns {{kind: 'dot', state: 'waiting'|'running', filled: boolean}|{kind: 'unseen'}|null} */
  function marker(activity, unseen) {
    if (activity === "waiting") return { kind: "dot", state: "waiting", filled: true };
    if (unseen > 0) return { kind: "unseen" };
    if (activity === "working") return { kind: "dot", state: "running", filled: false };
    return null;
  }

  // The rail has room for three repository names, and counts the rest.
  const NAMED = 3;

  /** @type {{initials?: string, shortcut?: string, name?: string, repos?: {name: string, role: string}[], live?: boolean, activity?: 'working'|'waiting'|'idle', unseen?: number, selected?: boolean, [attribute: string]: any}} */
  let {
    initials,
    shortcut = "",
    name,
    repos = [],
    live = false,
    activity = "idle",
    unseen = 0,
    selected = false,
    ...rest
  } = $props();

  let mark = $derived(marker(activity, unseen));
  // Pick order is a ranking, so dropping the tail drops the least important.
  let named = $derived(repos.slice(0, NAMED));
  let hidden = $derived(repos.length - named.length);
  let reason = $derived(
    activity === "waiting" ? "Waiting for you" : unseen > 0 ? `${unseen} unseen` : "",
  );
</script>

<!-- Taking focus on the press would leave the keyboard on the row rather than in
     the session it selects, including when that session is already the one shown. -->
<button
  type="button"
  class="item"
  class:selected
  class:cold={!selected && !live}
  onmousedown={(event) => event.preventDefault()}
  {...rest}>
  <div class="avatar" class:keyed={shortcut}>
    {shortcut || initials}
    {#if mark?.kind === "dot"}
      <StatusDot
        state={mark.state}
        size={8}
        style="position: absolute; top: -3px; right: -3px; outline: 2px solid var(--surface-chrome);
               background: {mark.filled ? 'var(--state-waiting)' : 'transparent'};
               border: {mark.filled ? 'none' : '1px solid var(--state-running)'}" />
    {:else if mark?.kind === "unseen"}
      <span class="unseen" aria-hidden="true">&#9670;</span>
    {/if}
  </div>

  <div class="text">
    <div class="name">{name}</div>
    <div class="reason" class:waiting={activity === "waiting"}>
      {#if reason}
        {reason}
      {:else if named.length}
        {#each named as repo, i (repo.name)}{i ? " · " : ""}<span
            class="repo"
            class:reference={repo.role === "reference"}>{repo.name}</span>{/each}{#if hidden}
          <span class="more">+{hidden}</span>
        {/if}
      {:else}
        no repositories yet
      {/if}
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

  .cold {
    opacity: 0.78;
  }

  .item:hover:not(.selected) {
    border-color: var(--border-default);
    background: var(--surface-raised);
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

  .unseen {
    position: absolute;
    top: -6px;
    right: -5px;
    font: 10px var(--font-machine);
    color: var(--state-guided);
    text-shadow: 0 0 3px var(--surface-chrome);
  }

  .text {
    min-width: 0;
    flex: 1;
  }

  .name,
  .reason {
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

  .reason {
    font: var(--machine-xs);
    font-size: 9.5px;
    margin-top: 2px;
    color: var(--text-faint);
  }

  .selected .reason {
    color: var(--text-muted);
  }

  .reason.waiting {
    color: var(--state-waiting);
  }

  .repo {
    color: var(--role-editing);
  }

  .repo.reference {
    color: var(--role-reference);
  }

  .more {
    color: var(--text-faint);
  }
</style>
