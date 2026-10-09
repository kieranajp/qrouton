<script>
  import { AGENT_ROLES, AGENT_STATES } from "../bridge/generated.js";
  import CapsLabel from "../core/CapsLabel.svelte";
  import {
    activeAgent,
    capabilityNote,
    duration,
    finishedAgent,
    hierarchy,
    providerLabel,
    recordLabel,
    roleLabel,
    runningRoot,
    stateLabel,
    subagentTally,
    typeLabel,
  } from "./activity.js";

  /** @type {{agents?: {provider?: string, attention_known?: boolean, children_known?: boolean, parents_known?: boolean, outcomes_known?: boolean, agents?: any[]}}} */
  let { agents = {} } = $props();

  let records = $derived(agents.agents ?? []);
  let ranks = $derived(hierarchy(records));
  let running = $derived(records.some(runningRoot));
  let note = $derived(capabilityNote(agents));

  const key = (record) => `${record.provider ?? ""}:${record.run_id ?? ""}:${record.id ?? ""}`;

  /** Which leads have had their subagents asked for. */
  let opened = $state(/** @type {Record<string, boolean>} */ ({}));
  const turn = (id) => (opened = { ...opened, [id]: !opened[id] });

  /** @param {any} record */
  function mark(record) {
    if (finishedAgent(record)) return record.state === AGENT_STATES.FAILED ? "failed" : "done";
    if (record.state === AGENT_STATES.WAITING && record.role === AGENT_ROLES.ORCHESTRATOR) return "waiting";
    return "running";
  }
</script>

<section class="activity" aria-labelledby="activity-heading">
  <h2 id="activity-heading"><CapsLabel tone="dim" centred>Activity</CapsLabel></h2>

  {#if !running && !ranks.roots.length}
    <p class="empty">No agent running</p>
  {/if}

  {#each ranks.roots as root (key(root.record))}
    <div class="rank">
      <div class="row" aria-label={recordLabel(root.record, agents)}>
        <span class="dot {mark(root.record)}" aria-hidden="true"></span>
        <span class="identity">
          <span class="who">{roleLabel(root.record.role) || "Agent"}</span>
          {#if providerLabel(root.record.provider || agents.provider, agents)}
            <span class="what">{providerLabel(root.record.provider || agents.provider, agents)}</span>
          {/if}
        </span>
        {#if stateLabel(root.record.state, root.record.role)}
          <span class="aside">{stateLabel(root.record.state, root.record.role)}</span>
        {/if}
      </div>

      {#each root.leads as lead (key(lead.record))}
        <div class="row lead" aria-label={recordLabel(lead.record, agents)}>
          <span class="dot {mark(lead.record)}" aria-hidden="true"></span>
          <span class="identity">
            <span class="who">{typeLabel(lead.record.type) || roleLabel(lead.record.role) || "Agent"}</span>
          </span>
          {#if duration(lead.record)}<span class="aside">{duration(lead.record)}</span>{/if}
        </div>

        {#if lead.subagents.length}
          <button
            type="button"
            class="disclose"
            aria-expanded={Boolean(opened[key(lead.record)])}
            onclick={() => turn(key(lead.record))}>
            <span class="tally">{subagentTally(lead.subagents)}</span>
            <span class="caret" class:open={opened[key(lead.record)]} aria-hidden="true"
              >{opened[key(lead.record)] ? "▲" : "▸"}</span>
          </button>
          {#if opened[key(lead.record)]}
            <div class="subagents">
              {#each lead.subagents as sub (key(sub))}
                <div class="row sub" aria-label={recordLabel(sub, agents)}>
                  <span class="dot small {mark(sub)}" aria-hidden="true"></span>
                  <span class="name" class:spent={finishedAgent(sub)}
                    >{typeLabel(sub.type) || "Subagent"}</span>
                  <span class="aside">{finishedAgent(sub) ? "done" : duration(sub)}</span>
                </div>
              {/each}
            </div>
          {/if}
        {/if}
      {/each}
    </div>
  {/each}

  {#if ranks.observed.length}
    <div class="observed">
      <div class="group-label">Observed agents</div>
      {#each ranks.observed as record (key(record))}
        <div class="row" aria-label={recordLabel(record, agents)}>
          {#if activeAgent(record)}<span class="dot {mark(record)}" aria-hidden="true"></span>{/if}
          <span class="identity">
            <span class="who">{typeLabel(record.type) || roleLabel(record.role) || "Agent"}</span>
          </span>
          {#if stateLabel(record.state, record.role)}
            <span class="aside">{stateLabel(record.state, record.role)}</span>
          {/if}
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

  /* Depth is an indent step. A tree rule down the left drew three ranks as a
     structure to read rather than three facts to glance at. */
  .rank {
    display: flex;
    flex-direction: column;
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

  .lead {
    padding-left: calc(15px * var(--ui-scale));
  }

  .sub {
    padding: calc(3px * var(--ui-scale)) 0;
  }

  .dot {
    flex: none;
    width: calc(7px * var(--ui-scale));
    height: calc(7px * var(--ui-scale));
    background: var(--state-running);
  }

  .dot.small {
    width: calc(6px * var(--ui-scale));
    height: calc(6px * var(--ui-scale));
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

  .disclose {
    display: flex;
    align-items: center;
    gap: calc(8px * var(--ui-scale));
    width: 100%;
    padding: calc(4px * var(--ui-scale)) 0 calc(4px * var(--ui-scale)) calc(30px * var(--ui-scale));
    background: transparent;
    border: 0;
    text-align: left;
    cursor: pointer;
    font: var(--machine-xs);
    font-size: calc(9.5px * var(--ui-scale));
    color: var(--text-faint);
  }

  .disclose:hover {
    color: var(--text-primary);
  }

  .tally {
    flex: 1;
    min-width: 0;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .caret {
    flex: none;
    font: var(--terminal-sm);
    color: var(--text-faint);
  }

  .caret.open {
    color: var(--accent-action);
  }

  .subagents {
    display: flex;
    flex-direction: column;
    padding-left: calc(30px * var(--ui-scale));
  }

  .name {
    flex: 1;
    min-width: 0;
    color: var(--text-secondary);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
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
