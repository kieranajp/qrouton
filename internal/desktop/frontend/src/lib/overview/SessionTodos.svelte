<script>
  import { dashboard } from "../dashboard.svelte.js";
  import PullChip from "../panes/PullChip.svelte";
  import { pullRefs } from "../panes/todo.js";
  import { openDocumentAt } from "./calls.js";
  import Tier from "./Tier.svelte";
  import { todoTier } from "./todos.js";

  /** @type {{slug: string, todos?: import("./todos.js").TodoItem[]}} */
  let { slug, todos = [] } = $props();

  const pulls = dashboard();
  let rows = $derived(todoTier(todos));
  /** @type {import("../dashboard/dashboard.js").PullRequest[]} */
  let tracked = $derived(pulls.status.sessions?.[slug]?.pullRequests ?? []);

  /** @param {import("./todos.js").TodoItem} item */
  async function open(item) {
    const opened = await openDocumentAt(slug, item.path, item.line);
    if (!opened.ok) console.error("open todo", item.path, opened.error);
  }
</script>

{#if rows.length}
  <Tier label="For you" name="todos">
    {#each rows as row (row.key)}
      {@const named = pullRefs(row.item, tracked)}
      <div class="todo" class:blocking={row.blocking} data-todo={row.item.text}>
        <button type="button" class="open" onclick={() => open(row.item)}>
          <span class="text">{row.item.text}</span>
          <span class="meta">
            {#if row.item.section}<span class="section">{row.item.section}</span>{/if}
            {#if named.where}<span class="where">Where {named.where}</span>{/if}
            {#if row.blocking}<span class="blocks">Blocks {row.item.blocks}</span>{/if}
          </span>
        </button>
        {#if named.refs.length}
          <span class="refs">
            {#each named.refs as ref (ref.url)}
              <PullChip {ref} />
            {/each}
          </span>
        {/if}
      </div>
    {/each}
  </Tier>
{/if}

<style>
  .todo {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    column-gap: calc(10px * var(--ui-scale));
    row-gap: calc(4px * var(--ui-scale));
    min-width: 0;
    padding: calc(3px * var(--ui-scale)) 0;
  }

  .open {
    display: flex;
    flex: 1 1 calc(220px * var(--ui-scale));
    flex-direction: column;
    gap: 2px;
    min-width: 0;
    padding: 0;
    border: 0;
    background: none;
    text-align: left;
    cursor: pointer;
  }

  .open:focus-visible {
    outline: 2px solid var(--accent-action);
    outline-offset: 2px;
  }

  .text {
    font: var(--machine-sm);
    color: var(--text-primary);
    overflow-wrap: anywhere;
  }

  .open:hover .text {
    text-decoration: underline;
  }

  .meta {
    display: flex;
    flex-wrap: wrap;
    column-gap: calc(8px * var(--ui-scale));
    font: var(--machine-xs);
    color: var(--text-muted);
  }

  .where {
    color: var(--text-faint);
    overflow-wrap: anywhere;
  }

  .blocks {
    color: var(--state-waiting);
  }

  .refs {
    display: flex;
    flex-wrap: wrap;
    gap: calc(6px * var(--ui-scale));
  }
</style>
