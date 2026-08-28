<script>
  import CapsLabel from "../core/CapsLabel.svelte";
  import {
    activeAgent,
    capabilityNote,
    parentLabel,
    projectAgents,
    providerLabel,
    recordLabel,
    roleLabel,
    runningRoot,
    stateLabel,
    typeLabel,
  } from "./activity.js";

  /** @type {{agents?: {provider?: string, attention_known?: boolean, children_known?: boolean, parents_known?: boolean, outcomes_known?: boolean, agents?: any[]}}} */
  let { agents = {} } = $props();

  let records = $derived(agents.agents ?? []);
  let projected = $derived(projectAgents(records));
  let running = $derived(records.some(runningRoot));
  let note = $derived(capabilityNote(agents));

  const key = (record) => `${record.provider ?? ""}:${record.run_id ?? ""}:${record.id ?? ""}`;
</script>

{#snippet treeAgent(node)}
  <li
    role="treeitem"
    aria-level={node.level}
    aria-selected="false"
    aria-label={recordLabel(node.record, agents.provider)}>
    <div class="record" title={recordLabel(node.record, agents.provider)}>
      {#if activeAgent(node.record)}<span class="agent-dot" aria-hidden="true">●</span>{/if}
      <div class="identity">
        <span class="role">{roleLabel(node.record.role)}</span>
        <span class="type">
          {node.record.role === "Orchestrator"
            ? providerLabel(node.record.provider || agents.provider)
            : typeLabel(node.record.type)}
        </span>
      </div>
      <span class="state">{stateLabel(node.record.state)}</span>
    </div>
    {#if node.children.length}
      <ul role="group">
        {#each node.children as child (key(child.record))}
          {@render treeAgent(child)}
        {/each}
      </ul>
    {/if}
  </li>
{/snippet}

<section class="activity" aria-labelledby="activity-heading">
  <h2 id="activity-heading"><CapsLabel tone="dim">Activity</CapsLabel></h2>

  {#if !running}
    <p class="empty">No agent running</p>
  {/if}

  {#if projected.trees.length}
    <ul class="tree" role="tree" aria-label="Agent hierarchy">
      {#each projected.trees as root (key(root.record))}
        {@render treeAgent(root)}
      {/each}
    </ul>
  {/if}

  {#if projected.observed.length}
    <div class="observed">
      <div class="group-label">Observed agents</div>
      <ul aria-label="Observed agents">
        {#each projected.observed as record (key(record))}
          <li aria-label={`${recordLabel(record, agents.provider)} · ${parentLabel(record)}`}>
            <div class="record" title={recordLabel(record, agents.provider)}>
              {#if activeAgent(record)}<span class="agent-dot" aria-hidden="true">●</span>{/if}
              <div class="identity">
                <span class="role">{roleLabel(record.role)}</span>
                <span class="type">{typeLabel(record.type)}</span>
              </div>
              <span class="state">{stateLabel(record.state)}</span>
            </div>
            <div class="parent" title={parentLabel(record)}>{parentLabel(record)}</div>
          </li>
        {/each}
      </ul>
    </div>
  {/if}

  {#if note}<p class="capability">{note}</p>{/if}
</section>

<style>
  .activity {
    display: flex;
    flex-direction: column;
    gap: 6px;
    padding-top: 10px;
    border-top: 1px solid var(--border-subtle);
  }

  h2,
  p,
  ul {
    margin: 0;
  }

  h2 {
    display: flex;
  }

  ul {
    list-style: none;
    padding: 0;
  }

  .tree [role="group"] {
    margin-left: 8px;
    padding-left: 7px;
    border-left: 1px solid var(--border-subtle);
  }

  .record {
    min-width: 0;
    display: flex;
    align-items: baseline;
    justify-content: space-between;
    gap: 5px;
    padding: 3px 0;
    font: var(--machine-xs);
    font-size: 9.5px;
  }

  .identity {
    min-width: 0;
    flex: 1;
    display: flex;
    flex-direction: column;
  }

  .agent-dot {
    flex: none;
    color: var(--state-running);
    font-size: 8px;
    line-height: 1;
  }

  .role {
    color: var(--text-secondary);
  }

  .type,
  .parent {
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    color: var(--text-faint);
  }

  .state {
    flex: none;
    color: var(--text-muted);
  }

  .empty,
  .capability,
  .parent,
  .group-label {
    font: var(--machine-xs);
    font-size: 9.5px;
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
