<script>
  import Icon from "../core/Icon.svelte";
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
      <a
        class="pull"
        href={row.url}
        title={row.fullRepo}
        data-state={row.state}
        onclick={(event) => (event.preventDefault(), openURL(row.url))}>
        <Icon name={row.icon} />
        <span class="number">{row.number}</span>
        <span class="title">{row.title}</span>
        <span class="repo">{row.repo}</span>
      </a>
    {/each}
  </Tier>
{/if}

<style>
  .pull {
    display: flex;
    min-width: 0;
    align-items: center;
    gap: calc(8px * var(--ui-scale));
    font: var(--machine-xs);
    color: var(--text-primary);
    text-decoration: none;
  }

  .pull:hover .title {
    text-decoration: underline;
  }

  .number {
    flex: none;
    color: var(--text-secondary);
  }

  .title {
    flex: 1;
    min-width: 0;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .repo {
    flex: none;
    color: var(--text-faint);
  }

  .note {
    font: var(--machine-xs);
    color: var(--text-faint);
  }
</style>
