<script>
  import { AGENT_TREE_EVENT, CHROME_AGENT_TREE } from "../bridge/generated.js";
  import { openDocument } from "../docked.svelte.js";
  import { Call, Events, call } from "../wails.js";
  import LeadRow from "./LeadRow.svelte";
  import SessionRepos from "./SessionRepos.svelte";
  import Tier from "./Tier.svelte";
  import { elapsed, nowLine, overview, rootWords } from "./tree.js";

  /** The session's Overview: the orchestrator, its leads in stage order, and
   * the session's repositories.
   * @type {{slug: string, active?: boolean, root?: string, repos?: any[], onAddRepos?: () => void}} */
  let { slug, active = false, root = "", repos = [], onAddRepos } = $props();

  /** @type {import("./tree.js").AgentTree | undefined} */
  let tree = $state();
  let now = $state(Date.now());

  // A derived compares by value, so a chrome update that keeps the slug does
  // not reload the tree and rebuild every row.
  let session = $derived(slug);

  $effect(() => {
    const current = session;
    let live = false;
    let gone = false;
    tree = undefined;
    const off = Events.On(AGENT_TREE_EVENT + current, (event) => {
      live = true;
      tree = event.data;
    });
    call(Call.ByName(CHROME_AGENT_TREE, current)).then((answer) => {
      if (answer.ok && !live && !gone && answer.value) tree = answer.value;
    });
    return () => {
      gone = true;
      off();
    };
  });

  $effect(() => {
    if (!active) return;
    now = Date.now();
    const timer = setInterval(() => (now = Date.now()), 1000);
    return () => clearInterval(timer);
  });

  let model = $derived(overview(tree?.nodes ?? []));
  let orchestrator = $derived(model.root);
  let tools = $derived(tree?.tools ?? true);
  let doing = $derived(orchestrator ? nowLine(orchestrator, root) : "");

  /** @param {string} path */
  async function read(path) {
    try {
      await openDocument(path);
    } catch {}
  }
</script>

<div class="overview" style:display={active ? "flex" : "none"}>
  <Tier label="Orchestrator" name="orchestrator">
    <div class="line">
      <span class="provider">{tree?.providerLabel ?? ""}</span>
      {#if orchestrator}
        <span class="words">{rootWords(orchestrator, model.leads, tools)}</span>
        {#if orchestrator.reason}<span class="reason">{orchestrator.reason}</span>{/if}
        {#if doing}<span class="doing">{doing}</span>{/if}
        <span class="time">{elapsed(orchestrator.started, orchestrator.finished, now)}</span>
      {/if}
    </div>
  </Tier>
  {#if tree?.coverage}<p class="coverage">{tree.coverage}</p>{/if}

  {#if model.leads.length}
    <Tier label="Leads" name="leads">
      {#each model.leads as lead (lead.node.run + ":" + lead.node.id)}
        <LeadRow branch={lead} {now} {root} {tools} onOpenDocument={read} />
      {/each}
    </Tier>
  {/if}

  <SessionRepos {repos} {onAddRepos} />
</div>

<style>
  .overview {
    flex: 1;
    min-height: 0;
    flex-direction: column;
    overflow: hidden auto;
    padding: var(--space-10) var(--space-6) var(--space-6);
    background: var(--surface-app);
    container: overview / inline-size;
  }

  .line {
    display: flex;
    min-width: 0;
    flex-direction: row;
    align-items: baseline;
    gap: calc(10px * var(--ui-scale));
    font: var(--machine-xs);
    color: var(--text-muted);
  }

  .provider {
    flex: none;
    font: var(--machine-sm);
    color: var(--text-primary);
  }

  .words,
  .time {
    flex: none;
  }

  .reason {
    color: var(--state-waiting);
  }

  .doing,
  .reason {
    min-width: 0;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .time {
    margin-left: auto;
    color: var(--text-faint);
  }

  .coverage {
    margin: 0 0 calc(12px * var(--ui-scale));
    font: var(--machine-xs);
    color: var(--text-faint);
  }

  @container overview (max-width: 27.5rem) {
    .line {
      flex-wrap: wrap;
      column-gap: calc(6px * var(--ui-scale));
    }
  }
</style>
