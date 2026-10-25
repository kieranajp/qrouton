<script>
  import AgentRow from "./AgentRow.svelte";
  import Icon from "../core/Icon.svelte";
  import { agentName, elapsed, isFinished, nowLine, stateIcon, stateWords, tokenLabel, wroteLine } from "./tree.js";

  /** One subagent: its head line, then what it is doing now and what it wrote.
   * @type {{branch: import("./tree.js").Branch, now: number, root?: string, tools?: boolean}} */
  let { branch, now, root = "", tools = true } = $props();

  let expanded = $state(false);

  let node = $derived(branch.node);
  let finished = $derived(isFinished(node.state));
  let title = $derived(agentName(node));
  let taskName = $derived(node.taskName || "");
  let icon = $derived(stateIcon(node.state));
  let words = $derived(stateWords(node, tools));
  let doing = $derived(nowLine(node, root));
  let wrote = $derived(wroteLine(node.files));

  /** @param {string} stamp */
  const clock = (stamp) => {
    const time = new Date(stamp);
    return Number.isNaN(time.getTime()) || time.getFullYear() < 1971
      ? ""
      : time.toLocaleTimeString([], { hour12: false });
  };
</script>

<div class="agent" class:finished data-agent-id={node.id}>
  <button
    type="button"
    class="head"
    aria-label="{title}{taskName ? ` · ${taskName}` : ""} · {words}"
    aria-expanded={expanded}
    onclick={() => (expanded = !expanded)}>
    {#if icon}
      <Icon name={icon} />
    {:else}
      <span class="slot"></span>
    {/if}
    <span class="who">{title}</span>
    {#if taskName}<span class="task-name" title={taskName}>· {taskName}</span>{/if}
    {#if node.description}<span class="what">{node.description}</span>{/if}
    {#if finished && wrote}<span class="what">wrote {wrote}</span>{/if}
    <span class="time">{elapsed(node.started, node.finished, now)}</span>
  </button>

  {#if !finished}
    <div class="lines">
      {#if doing}
        <span class="line"><span class="cue">now</span> {doing}</span>
      {:else}
        <span class="line cue">{words}</span>
      {/if}
      {#if wrote}<span class="line"><span class="cue">wrote</span> {wrote}</span>{/if}
    </div>
  {/if}

  {#if expanded}
    <div class="detail">
      {#each node.tools ?? [] as event, i (i)}
        <div class="event" class:failed={event.failed}>
          <span class="cue">{clock(event.start)}</span>
          <span>{nowLine(event, root)}</span>
          {#if event.failed}<span>failed</span>{:else if event.abandoned}<span class="cue">no result</span>{/if}
        </div>
      {/each}
      {#if tokenLabel(node.tokens)}<div class="cue">{tokenLabel(node.tokens)}</div>{/if}
      {#if node.reason}<div class="outcome">{node.reason}</div>{/if}
      {#if node.lastMessage}<div class="message">{node.lastMessage}</div>{/if}
    </div>
  {/if}

  {#if branch.children.length}
    <div class="children">
      {#each branch.children as child (child.node.id)}
        <AgentRow branch={child} {now} {root} {tools} />
      {/each}
    </div>
  {/if}
</div>

<style>
  .agent {
    display: flex;
    flex-direction: column;
    gap: calc(3px * var(--ui-scale));
    min-width: 0;
  }

  .head {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    column-gap: calc(8px * var(--ui-scale));
    width: 100%;
    min-width: 0;
    padding: 0;
    border: 0;
    background: none;
    text-align: left;
    cursor: pointer;
    font: var(--machine-xs);
    color: var(--text-muted);
  }

  .slot {
    flex: none;
    width: calc(16px * var(--ui-scale));
  }

  .who {
    flex: none;
    font: var(--machine-sm);
    color: var(--text-primary);
  }

  .task-name {
    flex: 1 1 0;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    color: var(--text-muted);
  }

  .what {
    flex: 1 1 0;
    min-width: 0;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .time {
    flex: none;
    margin-left: auto;
    color: var(--text-faint);
  }

  .finished .head,
  .finished .who {
    font: var(--machine-xs);
    color: var(--text-faint);
  }

  .lines,
  .detail {
    display: flex;
    flex-direction: column;
    padding-left: calc(24px * var(--ui-scale));
    font: var(--machine-xs);
    color: var(--text-secondary);
    min-width: 0;
  }

  .line {
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .cue {
    color: var(--text-faint);
  }

  .event.failed {
    color: var(--state-failed);
  }

  .outcome {
    color: var(--state-failed);
  }

  .message {
    white-space: pre-wrap;
    color: var(--text-muted);
  }

  .children {
    display: flex;
    flex-direction: column;
    gap: calc(10px * var(--ui-scale));
    padding: calc(4px * var(--ui-scale)) 0 0 calc(24px * var(--ui-scale));
  }

  /* Narrow, a subagent's description takes the line under its name. */
  @container overview (max-width: 27.5rem) {
    .what {
      order: 3;
      flex-basis: 100%;
      padding-left: calc(24px * var(--ui-scale));
    }
  }
</style>
