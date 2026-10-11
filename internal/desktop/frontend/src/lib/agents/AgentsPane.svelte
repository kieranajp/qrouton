<script>
  import { AGENT_TREE_EVENT, CHROME_AGENT_TREE, REPO_ROLES } from "../bridge/generated.js";
  import Button from "../core/Button.svelte";
  import { openDocument } from "../docked.svelte.js";
  import { GLYPHS, READ_ONLY } from "../roles.js";
  import { Call, Events, call } from "../wails.js";
  import LeadRow from "./LeadRow.svelte";
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
  <section class="tier orchestrator" aria-label="Orchestrator">
    <span class="label">Orchestrator</span>
    <div class="body line">
      <span class="provider">{tree?.providerLabel ?? ""}</span>
      {#if orchestrator}
        <span class="words">{rootWords(orchestrator, model.leads, tools)}</span>
        {#if orchestrator.reason}<span class="reason">{orchestrator.reason}</span>{/if}
        {#if doing}<span class="doing">{doing}</span>{/if}
        <span class="time">{elapsed(orchestrator.started, orchestrator.finished, now)}</span>
      {/if}
    </div>
  </section>
  {#if tree?.coverage}<p class="coverage">{tree.coverage}</p>{/if}

  {#if model.leads.length}
    <section class="tier leads" aria-label="Leads">
      <span class="label">Leads</span>
      <div class="body">
        {#each model.leads as lead (lead.node.run + ":" + lead.node.id)}
          <LeadRow branch={lead} {now} {root} {tools} onOpenDocument={read} />
        {/each}
      </div>
    </section>
  {/if}

  <section class="tier session" aria-label="This session">
    <span class="label">This session</span>
    <div class="body">
      {#if repos.length}
        <div class="repos">
          {#each repos as repo (repo.name)}
            <div class="repo">
              <span class="repo-name {repo.role}">{GLYPHS[repo.role]} {repo.name}</span>
              {#if repo.role === REPO_ROLES.REFERENCE}
                <span class="stat">{READ_ONLY}</span><span></span>
              {:else if repo.measured === false}
                <span class="stat">unmeasured</span><span></span>
              {:else}
                <span class="stat">{repo.commits} commit{repo.commits === 1 ? "" : "s"}</span>
                <span class="diff"
                  ><span class="added">+{repo.insertions ?? 0}</span>
                  <span class="removed">−{repo.deletions ?? 0}</span></span>
              {/if}
            </div>
          {/each}
        </div>
      {:else}
        <span class="empty">no repositories yet</span>
      {/if}
      <span class="add"><Button variant="cube" size="sm" glyph="+" wide onclick={onAddRepos}>Add repos</Button></span>
    </div>
  </section>
</div>

<style>
  .overview {
    flex: 1;
    min-height: 0;
    flex-direction: column;
    overflow: hidden auto;
    padding: calc(14px * var(--ui-scale));
    background: var(--surface-app);
    container: overview / inline-size;
  }

  .tier {
    display: flex;
    gap: calc(10px * var(--ui-scale));
    min-width: 0;
  }

  .tier + .tier {
    border-top: 1px solid var(--border-subtle);
    padding-top: calc(12px * var(--ui-scale));
  }

  .orchestrator {
    padding-bottom: calc(12px * var(--ui-scale));
  }

  .leads {
    padding-bottom: calc(12px * var(--ui-scale));
  }

  .session {
    margin-top: auto;
  }

  .label {
    flex: none;
    width: calc(108px * var(--ui-scale));
    padding-top: calc(4px * var(--ui-scale));
    font: var(--instruction);
    letter-spacing: var(--instruction-tracking);
    text-transform: uppercase;
    white-space: nowrap;
    color: var(--text-faint);
  }

  .orchestrator .label {
    padding-top: 0;
  }

  .body {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .line {
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

  .session .body {
    gap: calc(10px * var(--ui-scale));
  }

  .repos {
    display: grid;
    grid-template-columns: minmax(0, 1fr) auto auto;
    align-items: baseline;
    gap: calc(6px * var(--ui-scale)) calc(16px * var(--ui-scale));
    font: var(--machine-xs);
  }

  .repo {
    display: contents;
  }

  .repo-name {
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .repo-name.editing {
    color: var(--role-editing);
  }

  .repo-name.reference {
    color: var(--role-reference);
  }

  .stat,
  .empty {
    color: var(--text-faint);
  }

  .empty {
    font: var(--machine-xs);
  }

  .diff {
    text-align: right;
  }

  .added {
    color: var(--state-success);
  }

  .removed {
    color: var(--state-failed);
  }

  .add {
    max-width: calc(200px * var(--ui-scale));
    padding: 0 calc(3px * var(--ui-scale)) calc(3px * var(--ui-scale)) 0;
  }

  /* Narrow, each tier's label heads its content, the way the rail stacks
     them, and a repository takes two lines. */
  @container overview (max-width: 27.5rem) {
    .tier {
      flex-direction: column;
      gap: calc(6px * var(--ui-scale));
    }

    .label {
      width: auto;
      padding-top: 0;
    }

    .orchestrator .line {
      flex-wrap: wrap;
      column-gap: calc(6px * var(--ui-scale));
    }

    .repos {
      display: flex;
      flex-direction: column;
      gap: 0;
    }

    .repo {
      display: flex;
      flex-wrap: wrap;
      column-gap: calc(6px * var(--ui-scale));
      padding: calc(5px * var(--ui-scale)) 0;
    }

    .repo-name {
      flex-basis: 100%;
    }

    .stat,
    .diff {
      font-size: calc(9.5px * var(--ui-scale));
    }
  }
</style>
