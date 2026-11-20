<script>
  import { uiScale } from "../scale.svelte.js";
  import SessionCard from "./SessionCard.svelte";
  import { COLUMN_GAP, DASHBOARD_TITLE, columnCount, estimatedHeight, masonry } from "./dashboard.js";

  /** @type {{sessions: any[]}} */
  let { sessions = [] } = $props();

  let width = $state(0);
  let columns = $derived(
    masonry(
      sessions.map(() => estimatedHeight()),
      columnCount(width, uiScale() / 100),
    ),
  );
</script>

<section class="dashboard" aria-labelledby="dashboard-title">
  <!-- Measured here rather than on the grid, whose height changes with the count it is measured for. -->
  <header bind:clientWidth={width}>
    <div></div>
    <h1 id="dashboard-title">{DASHBOARD_TITLE}</h1>
    <div class="actions"></div>
  </header>

  <div class="grid" style:--gap="{COLUMN_GAP}px">
    {#each columns as column, c (c)}
      <div class="column">
        {#each column as index (sessions[index].slug)}
          <SessionCard row={sessions[index]} position={index} />
        {/each}
      </div>
    {/each}
  </div>
</section>

<style>
  .dashboard {
    flex: 1;
    min-width: 0;
    box-sizing: border-box;
    display: flex;
    flex-direction: column;
    gap: calc(22px * var(--ui-scale));
    padding: calc(24px * var(--ui-scale)) calc(30px * var(--ui-scale));
    overflow: hidden auto;
    /* A scrollbar that came and went with the card count would change the width the columns are counted from. */
    scrollbar-gutter: stable;
    background: var(--surface-app);
    border-left: 1px solid var(--border-subtle);
  }

  header {
    display: grid;
    grid-template-columns: 1fr auto 1fr;
    align-items: center;
  }

  h1 {
    margin: 0;
    font: var(--display-md);
    letter-spacing: var(--display-tracking);
    color: var(--text-primary);
  }

  .actions {
    justify-self: end;
  }

  .grid {
    display: flex;
    align-items: flex-start;
    gap: calc(var(--gap) * var(--ui-scale));
  }

  .column {
    flex: 1 1 0;
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: calc(var(--gap) * var(--ui-scale));
  }
</style>
