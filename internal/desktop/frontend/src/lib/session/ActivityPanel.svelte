<script>
  import { AGENT_STATES } from "../bridge/generated.js";
  import CapsLabel from "../core/CapsLabel.svelte";
  import {
    activeAgent,
    capabilityNote,
    duration,
    finishedAgent,
    isRoot,
    projectAgents,
    providerLabel,
    recordLabel,
    rows,
    runningRoot,
    stateLabel,
    typeLabel,
  } from "./activity.js";

  /** @type {{agents?: {provider?: string, attention_known?: boolean, children_known?: boolean, parents_known?: boolean, outcomes_known?: boolean, agents?: any[]}}} */
  let { agents = {} } = $props();

  let records = $derived(agents.agents ?? []);
  let tree = $derived(projectAgents(records));
  let lines = $derived(rows(tree.trees));
  let running = $derived(records.some(runningRoot));
  let note = $derived(capabilityNote(agents));

  const key = (record) => `${record.provider ?? ""}:${record.run_id ?? ""}:${record.id ?? ""}`;

  /** @param {any} record */
  function mark(record) {
    if (finishedAgent(record)) return record.state === AGENT_STATES.FAILED ? "failed" : "done";
    if (record.state === AGENT_STATES.WAITING && isRoot(record)) return "waiting";
    return "running";
  }
</script>

<section class="activity" aria-labelledby="activity-heading">
  <h2 id="activity-heading"><CapsLabel tone="dim" centred>Activity</CapsLabel></h2>

  {#if !running && !lines.length}
    <p class="empty">No agent running</p>
  {/if}

  {#each lines as line (key(line.record))}
    <div
      class="row"
      style:padding-left="calc({(line.level - 1) * 15}px * var(--ui-scale))"
      aria-label={recordLabel(line.record, agents)}>
      <span class="dot {mark(line.record)}" aria-hidden="true"></span>
      <span class="identity">
        {#if isRoot(line.record)}
          <span class="who">Orchestrator</span>
          {#if providerLabel(line.record.provider || agents.provider, agents)}
            <span class="what">{providerLabel(line.record.provider || agents.provider, agents)}</span>
          {/if}
        {:else}
          <span class="who" class:spent={finishedAgent(line.record)}>{typeLabel(line.record.type) || "Agent"}</span>
          {#if line.record.description}<span class="what">{line.record.description}</span>{/if}
        {/if}
      </span>
      <span class="aside">
        {isRoot(line.record) ? stateLabel(line.record.state, true) : finishedAgent(line.record) ? stateLabel(line.record.state) : duration(line.record)}
      </span>
    </div>
  {/each}

  {#if tree.observed.length}
    <div class="observed">
      <div class="group-label">Observed agents</div>
      {#each tree.observed as record (key(record))}
        <div class="row" aria-label={recordLabel(record, agents)}>
          {#if activeAgent(record)}<span class="dot {mark(record)}" aria-hidden="true"></span>{/if}
          <span class="identity">
            <span class="who">{typeLabel(record.type) || "Agent"}</span>
          </span>
          {#if stateLabel(record.state)}<span class="aside">{stateLabel(record.state)}</span>{/if}
        </div>
      {/each}
    </div>
  {/if}

  {#if note}<p class="capability">{note}</p>{/if}
</section>

<style>
  .activity {
    display: flex;
    flex-direction: column;
    gap: calc(6px * var(--ui-scale));
    padding-top: calc(10px * var(--ui-scale));
    border-top: 1px solid var(--border-subtle);
  }

  h2,
  p {
    margin: 0;
  }

  .row {
    min-width: 0;
    display: flex;
    align-items: baseline;
    gap: calc(8px * var(--ui-scale));
    padding: calc(5px * var(--ui-scale)) 0;
    font: var(--machine-xs);
    font-size: calc(9.5px * var(--ui-scale));
  }

  .dot {
    flex: none;
    width: calc(7px * var(--ui-scale));
    height: calc(7px * var(--ui-scale));
    background: var(--state-running);
  }

  .dot.waiting {
    background: var(--state-waiting);
  }

  .dot.failed {
    background: var(--state-failed);
  }

  .dot.done {
    background: transparent;
    box-shadow: inset 0 0 0 1px var(--ctp-surface-2);
  }

  .identity {
    min-width: 0;
    flex: 1;
    display: flex;
    flex-direction: column;
    gap: 1px;
  }

  .who {
    font-size: calc(10.5px * var(--ui-scale));
    color: var(--text-primary);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .what {
    color: var(--text-faint);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .aside {
    flex: none;
    color: var(--text-muted);
  }

  .spent {
    color: var(--text-faint);
  }

  .empty,
  .capability,
  .group-label {
    font: var(--machine-xs);
    font-size: calc(9.5px * var(--ui-scale));
    color: var(--text-faint);
  }

  .group-label {
    margin-top: 2px;
    color: var(--text-muted);
  }

  .capability {
    line-height: 1.45;
  }
</style>
