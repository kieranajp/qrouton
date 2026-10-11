<script>
  import AgentRow from "./AgentRow.svelte";
  import { elapsed, isFinished, stateTone, stateWords, tokenLabel, toolLine, typeLabel } from "./tree.js";

  /** @type {{branch: import("./tree.js").Branch, depth?: number, now: number, tools?: boolean}} */
  let { branch, depth = 0, now, tools = true } = $props();

  let expanded = $state(false);
  let showFinished = $state(false);

  let node = $derived(branch.node);
  let finished = $derived(isFinished(node.state));
  let words = $derived(stateWords(node, tools));
  let title = $derived(typeLabel(node.type) || "Agent");
  let live = $derived(branch.children.filter((child) => !isFinished(child.node.state)));
  let spent = $derived(branch.children.filter((child) => isFinished(child.node.state)));

  /** @param {string} stamp */
  const clock = (stamp) => {
    const time = new Date(stamp);
    return Number.isNaN(time.getTime()) || time.getFullYear() < 1971
      ? ""
      : time.toLocaleTimeString([], { hour12: false });
  };
</script>

<div class="branch">
  <button
    type="button"
    class="row"
    class:spent={finished}
    style:padding-left="calc({depth * 16 + 4}px * var(--ui-scale))"
    aria-label="{title} · {words}"
    aria-expanded={expanded}
    onclick={() => (expanded = !expanded)}>
    <span class="dot" style:background="var({stateTone(node.state)})" aria-hidden="true"></span>
    <span class="who">{title}</span>
    {#if node.description}<span class="what">{node.description}</span>{/if}
    <span class="words">{words}</span>
    {#if node.tool}
      <span class="tool">{toolLine(node)}</span>
    {/if}
    <span class="meta">{elapsed(node.started, node.finished, now)}</span>
    {#if node.toolCount}<span class="meta">{node.toolCount} tools</span>{/if}
    {#if finished && tokenLabel(node.tokens)}<span class="meta">{tokenLabel(node.tokens)}</span>{/if}
  </button>

  {#if expanded}
    <div class="detail" style:margin-left="calc({depth * 16 + 20}px * var(--ui-scale))">
      {#each node.tools ?? [] as event, i (i)}
        <div class="event" class:failed={event.failed}>
          <span class="clock">{clock(event.start)}</span>
          <span>{event.tool}</span>
          <span class="target">{event.target}</span>
          {#if event.failed}<span>failed</span>{/if}
        </div>
      {/each}
      {#if node.files?.length}
        <div class="label">Files written</div>
        {#each node.files as file (file)}<div class="target">{file}</div>{/each}
      {/if}
      {#if finished && node.reason}<div class="outcome">{node.reason}</div>{/if}
      {#if finished && node.lastMessage}<div class="message">{node.lastMessage}</div>{/if}
    </div>
  {/if}

  {#each live as child (child.node.id)}
    <AgentRow branch={child} depth={depth + 1} {now} {tools} />
  {/each}

  {#if spent.length}
    <button
      type="button"
      class="toggle"
      style:padding-left="calc({(depth + 1) * 16 + 4}px * var(--ui-scale))"
      aria-expanded={showFinished}
      onclick={() => (showFinished = !showFinished)}>
      {spent.length} finished
    </button>
    {#if showFinished}
      {#each spent as child (child.node.id)}
        <AgentRow branch={child} depth={depth + 1} {now} {tools} />
      {/each}
    {/if}
  {/if}
</div>

<style>
  .row,
  .toggle {
    display: flex;
    align-items: baseline;
    gap: calc(8px * var(--ui-scale));
    width: 100%;
    min-width: 0;
    padding-block: calc(4px * var(--ui-scale));
    padding-right: calc(4px * var(--ui-scale));
    border: 0;
    background: none;
    color: var(--text-primary);
    font: var(--machine-xs);
    text-align: left;
    cursor: pointer;
  }

  .toggle {
    color: var(--text-muted);
  }

  .spent .who {
    color: var(--text-faint);
  }

  .dot {
    flex: none;
    width: calc(7px * var(--ui-scale));
    height: calc(7px * var(--ui-scale));
  }

  .who {
    flex: none;
  }

  .what {
    flex: 1 1 0;
    min-width: 0;
    color: var(--text-muted);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .words,
  .meta,
  .tool {
    flex: none;
    color: var(--text-muted);
  }

  .tool {
    max-width: 40%;
    min-width: 0;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .target {
    min-width: 0;
    color: var(--text-faint);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .detail {
    display: flex;
    flex-direction: column;
    gap: calc(2px * var(--ui-scale));
    padding: calc(4px * var(--ui-scale)) calc(8px * var(--ui-scale));
    border-left: 1px solid var(--border-subtle);
    font: var(--machine-xs);
    color: var(--text-muted);
  }

  .event {
    display: flex;
    gap: calc(8px * var(--ui-scale));
    min-width: 0;
  }

  .failed {
    color: var(--state-failed);
  }

  .clock,
  .label {
    color: var(--text-faint);
  }

  .outcome,
  .message {
    margin-top: calc(4px * var(--ui-scale));
    color: var(--text-primary);
  }

  .message {
    white-space: pre-wrap;
    overflow-wrap: anywhere;
  }
</style>
