<script>
  import StatusDot from "../core/StatusDot.svelte";

  /** @returns {{kind: 'dot', state: 'waiting'|'running', filled: boolean}|{kind: 'unseen'}|null} */
  function marker(activity, unseen) {
    if (activity === "waiting") return { kind: "dot", state: "waiting", filled: true };
    if (unseen > 0) return { kind: "unseen" };
    if (activity === "working") return { kind: "dot", state: "running", filled: false };
    return null;
  }

  /** @type {{initials?: string, name?: string, mode?: string, repos?: number, live?: boolean, activity?: 'working'|'waiting'|'idle', unseen?: number, selected?: boolean, [attribute: string]: any}} */
  let {
    initials,
    name,
    mode = "RPI",
    repos = 0,
    live = false,
    activity = "idle",
    unseen = 0,
    selected = false,
    ...rest
  } = $props();

  let mark = $derived(marker(activity, unseen));
  // Mode reads as one word: the reason line has ~19 monospace characters before
  // it truncates, and unlike the name above it, it is short enough not to.
  let modeLabel = $derived(mode === "RPI" ? "Guided" : "Open");
  let reason = $derived(
    activity === "waiting"
      ? "Waiting for you"
      : unseen > 0
        ? `${unseen} unseen`
        : `${modeLabel} · ${repos} repo${repos === 1 ? "" : "s"}`,
  );
</script>

<div class="item" class:selected class:cold={!selected && !live} {...rest}>
  <div class="avatar">
    {initials}
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
    <div class="reason" class:waiting={activity === "waiting"}>{reason}</div>
  </div>
</div>

<style>
  .item {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 8px 9px;
    cursor: pointer;
    border: 1px solid transparent;
    background: transparent;
  }

  .cold {
    opacity: 0.78;
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
</style>
