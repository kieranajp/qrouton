<script>
  import { AGENT_TREE_EVENT, CHROME_AGENT_TREE } from "../bridge/generated.js";
  import { Call, Events, call } from "../wails.js";
  import AgentRow from "./AgentRow.svelte";
  import { buildRuns, stateTone, stateWords, toolLine } from "./tree.js";

  /** @type {{slug: string, active?: boolean}} */
  let { slug, active = false } = $props();

  /** @type {import("./tree.js").AgentTree | undefined} */
  let tree = $state();
  let now = $state(Date.now());
  let showPrevious = $state(false);

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

  let runs = $derived(buildRuns(tree?.nodes ?? []));
  let current = $derived(runs[0]);
  let previous = $derived(runs.slice(1));
  let root = $derived(current?.root);
</script>

<div class="agents" style:display={active ? "flex" : "none"}>
  {#if tree}
    <header class="head">
      <span class="provider">{tree.providerLabel}</span>
      {#if root}
        <span class="dot" style:background="var({stateTone(root.state)})" aria-hidden="true"></span>
        <span class="words">{stateWords(root)}</span>
        {#if root.reason}<span class="muted">{root.reason}</span>{/if}
        {#if root.tool}
          <span class="muted">{toolLine(root)}</span>
        {/if}
      {/if}
    </header>
    {#if tree.coverage}<p class="coverage">{tree.coverage}</p>{/if}

    {#each current?.children ?? [] as branch (branch.node.id)}
      <AgentRow {branch} {now} />
    {/each}

    {#if previous.length}
      <button
        type="button"
        class="previous"
        aria-expanded={showPrevious}
        onclick={() => (showPrevious = !showPrevious)}>
        Previous run
      </button>
      {#if showPrevious}
        {#each previous as run (run.run)}
          {#if run.root}<AgentRow branch={{ node: run.root, children: run.children }} {now} />{/if}
        {/each}
      {/if}
    {/if}
  {/if}
</div>

<style>
  .agents {
    flex: 1;
    min-height: 0;
    flex-direction: column;
    overflow: hidden auto;
    padding: calc(12px * var(--ui-scale));
    background: var(--surface-app);
  }

  .head {
    display: flex;
    align-items: baseline;
    gap: calc(8px * var(--ui-scale));
    margin-bottom: calc(8px * var(--ui-scale));
    font: var(--machine-sm);
    color: var(--text-primary);
  }

  .dot {
    flex: none;
    width: calc(7px * var(--ui-scale));
    height: calc(7px * var(--ui-scale));
  }

  .muted,
  .coverage {
    color: var(--text-muted);
  }

  .coverage {
    margin: 0 0 calc(8px * var(--ui-scale));
    font: var(--machine-xs);
  }

  .previous {
    margin-top: calc(8px * var(--ui-scale));
    padding: calc(4px * var(--ui-scale)) 0;
    border: 0;
    background: none;
    color: var(--text-muted);
    font: var(--machine-xs);
    text-align: left;
    cursor: pointer;
  }
</style>
