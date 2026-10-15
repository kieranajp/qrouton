<script>
  import Icon from "../core/Icon.svelte";
  import StageSquare from "../overview/StageSquare.svelte";
  import { stageTone } from "../workflow.js";
  import { repositoryLine, summaryFacts } from "./rail.js";
  let { initials = "", shortcut = "", name = "", repos = [], summary = {}, unseen = 0, idle = "", selected = false, context = "" } = $props();
  let repository = $derived(repositoryLine(repos));
  let facts = $derived(summaryFacts(summary, unseen, idle));
</script>
<div class="identity" class:selected class:running={Boolean(summary.running)}>
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
          <span class="fact {fact.kind}" title={fact.title}>
            {#if fact.kind === "agents" && fact.stage}
              <span class="stage"><StageSquare tone={stageTone(fact.stage)} done live /></span>
            {:else if fact.kind === "agents" && fact.active}<Icon name="running" />{/if}
            {fact.label}
          </span>
        {/each}
      </div>
    </div>
{#if context}<span class="context">{context}</span>{/if}
</div>
<style>
.identity { display:flex; align-items:center; gap:calc(10px * var(--ui-scale)); min-width:0; flex:1; }
.context { font:var(--machine-xs); color:var(--text-muted); overflow-wrap:anywhere; }
  .avatar {
    position: relative;
    width: calc(30px * var(--ui-scale));
    height: calc(30px * var(--ui-scale));
    flex: none;
    display: flex;
    align-items: center;
    justify-content: center;
    font: 700 calc(13px * var(--ui-scale)) var(--font-machine);
    background: transparent;
    border: 1px solid var(--border-subtle);
    color: var(--text-faint);
  }

  .identity.running .avatar {
    background: var(--surface-raised);
    border-color: var(--border-default);
    color: var(--text-secondary);
  }

  .identity.selected .avatar {
    background: var(--accent-action);
    border-color: var(--accent-action);
    color: var(--text-on-accent);
  }

  .keyed {
    font: var(--machine-md);
    font-size: calc(11px * var(--ui-scale));
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
    font-size: calc(11px * var(--ui-scale));
    color: var(--text-muted);
  }

  .identity.running .name {
    color: var(--text-secondary);
  }

  .identity.selected .name {
    font: var(--machine-bold);
    font-size: calc(11px * var(--ui-scale));
    color: var(--text-primary);
  }

  .repositories,
  .facts {
    font: var(--machine-xs);
    font-size: calc(9.5px * var(--ui-scale));
    margin-top: 2px;
    color: var(--text-faint);
  }

  .identity.selected .repositories,
  .identity.selected .facts {
    color: var(--text-muted);
  }

  .extra {
    margin-left: 0.5ch;
    color: var(--text-muted);
  }

  .facts {
    display: flex;
    flex-wrap: wrap;
    column-gap: calc(6px * var(--ui-scale));
    row-gap: 1px;
  }

  .fact {
    flex: none;
    white-space: nowrap;
    display: inline-flex;
    align-items: center;
    gap: calc(3px * var(--ui-scale));
  }

  .stage {
    flex: none;
    display: grid;
    place-items: center;
    width: calc(16px * var(--ui-scale));
    height: calc(16px * var(--ui-scale));
  }

  .fact.attention {
    color: var(--state-waiting);
  }

  .fact.unseen {
    color: var(--state-guided);
  }

</style>
