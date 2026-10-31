<script>
  import { openURL } from "../wails.js";
  import { pullRows } from "./pulls.js";
  import Tier from "./Tier.svelte";

  /** @type {{pullRequests?: import("./pulls.js").PullRequestList}} */
  let { pullRequests } = $props();

  let shown = $derived(pullRows(pullRequests));
</script>

{#if shown}
  <Tier label="Pull requests" name="pulls">
    {#if shown.note}
      <span class="note">{shown.note}</span>
    {/if}
    {#each shown.rows as row (row.key)}
      <a class="pull" href={row.url} onclick={(event) => (event.preventDefault(), openURL(row.url))}>
        <span class="title">{row.title}</span>
        <span class="number">{row.number}</span>
      </a>
    {/each}
  </Tier>
{/if}

<style>
  .pull {
    display: flex;
    min-width: 0;
    align-items: baseline;
    gap: calc(10px * var(--ui-scale));
    font: var(--machine-xs);
    color: var(--text-primary);
    text-decoration: none;
  }

  .pull:hover .title {
    text-decoration: underline;
  }

  .title {
    min-width: 0;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .number {
    flex: none;
    margin-left: auto;
    color: var(--text-faint);
  }

  .note {
    font: var(--machine-xs);
    color: var(--text-faint);
  }
</style>
