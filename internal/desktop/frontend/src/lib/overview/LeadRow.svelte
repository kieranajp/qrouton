<script>
  import { AGENT_STATES } from "../bridge/generated.js";
  import ArtifactTag from "../core/ArtifactTag.svelte";
  import Icon from "../core/Icon.svelte";
  import { stageTone } from "../workflow.js";
  import AgentRow from "./AgentRow.svelte";
  import StageSquare from "./StageSquare.svelte";
  import { agentName, elapsed, isFinished, nowLine, stageName, stateTone, stateWords } from "./tree.js";

  /** One lead: its stage square and name, its document, and its subagents
   * while open. Only a live lead opens on its own.
   * @type {{branch: import("./tree.js").Branch, now: number, root?: string, tools?: boolean, onOpenDocument?: (path: string) => void}} */
  let { branch, now, root = "", tools = true, onOpenDocument } = $props();

  let node = $derived(branch.node);
  let finished = $derived(isFinished(node.state));
  /** @type {boolean | undefined} */
  let chosen = $state();
  let open = $derived(chosen ?? !finished);
  let name = $derived(stageName(node.stage) || agentName(node));
  let taskName = $derived(node.taskName || "");
  let tone = $derived(stageTone(node.stage) ?? `var(${stateTone(node.state)})`);
  let document = $derived(node.documents?.at(-1));
  let count = $derived(branch.children.length);
  let doing = $derived(nowLine(node, root) || stateWords(node, tools));

  function toggle() {
    if (count) chosen = !open;
  }
</script>

<div class="lead" class:live={!finished} data-agent-id={node.id}>
  <div class="row">
    <button
      type="button"
      class="toggle"
      class:leaf={!count}
      aria-label="{name}{taskName ? ` · ${taskName}` : ""} · {finished ? stateWords(node, tools) : doing}"
      aria-expanded={count ? open : undefined}
      onclick={toggle}>
      <StageSquare {tone} done={finished && node.state !== AGENT_STATES.FAILED && node.state !== AGENT_STATES.ENDED} live={!finished} />
      <span class="name">{name}</span>
      {#if taskName}<span class="task-name" title={taskName}>· {taskName}</span>{/if}
      {#if !finished}<span class="doing">{doing}</span>{/if}
    </button>
    {#if document}
      <button
        type="button"
        class="document"
        aria-label="Open {document.name}"
        onclick={() => onOpenDocument?.(document.path)}><ArtifactTag kind={document.kind} id={document.id} /></button>
    {/if}
    <span class="facts">
      {#if finished && count}<span class="count">{count} {count === 1 ? "subagent" : "subagents"} ·</span>{/if}
      {elapsed(node.started, node.finished, now)}
    </span>
    {#if count}
      <button type="button" class="disclosure" tabindex="-1" aria-hidden="true" onclick={toggle}>
        <Icon name={open ? "disclosure-open" : "disclosure-closed"} />
      </button>
    {:else}
      <span class="no-disclosure"></span>
    {/if}
  </div>

  {#if open && count}
    <div class="subagents">
      {#each branch.children as child (child.node.id)}
        <AgentRow branch={child} {now} {root} {tools} />
      {/each}
    </div>
  {/if}
</div>

<style>
  .lead {
    display: flex;
    flex-direction: column;
    min-width: 0;
  }

  .row {
    display: flex;
    align-items: center;
    gap: calc(8px * var(--ui-scale));
    padding: calc(3px * var(--ui-scale)) 0;
    min-width: 0;
    font: var(--machine-xs);
    color: var(--text-faint);
  }

  .toggle {
    display: flex;
    align-items: center;
    gap: calc(8px * var(--ui-scale));
    flex: 1 1 auto;
    min-width: 0;
    padding: 0;
    border: 0;
    background: none;
    text-align: left;
    cursor: pointer;
    font: var(--machine-xs);
    color: var(--text-muted);
  }

  .toggle.leaf {
    cursor: default;
  }

  .name {
    flex: none;
    font: var(--machine-sm);
    color: var(--text-secondary);
  }

  .task-name {
    flex: 0 1 auto;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    color: var(--text-muted);
  }

  .live .name {
    font: var(--machine-bold);
    font-size: calc(12.5px * var(--ui-scale));
    color: var(--text-primary);
  }

  .doing {
    min-width: 0;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .document,
  .disclosure {
    flex: none;
    display: flex;
    padding: 0;
    border: 0;
    background: none;
    cursor: pointer;
  }

  .disclosure {
    color: inherit;
  }

  .no-disclosure {
    flex: none;
    width: calc(16px * var(--ui-scale));
  }

  .facts {
    flex: none;
    margin-left: auto;
    white-space: nowrap;
  }

  .subagents {
    display: flex;
    flex-direction: column;
    gap: calc(12px * var(--ui-scale));
    padding: calc(8px * var(--ui-scale)) 0 calc(4px * var(--ui-scale)) calc(17px * var(--ui-scale));
  }

  /* Narrow, a finished lead drops its subagent count. */
  @container overview (max-width: 27.5rem) {
    .count {
      display: none;
    }
  }
</style>
