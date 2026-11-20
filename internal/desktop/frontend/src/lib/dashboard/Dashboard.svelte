<script module>
  import { SvelteSet } from "svelte/reactivity";

  const expanded = new SvelteSet();
</script>

<script>
  import Button from "../core/Button.svelte";
  import { uiScale } from "../scale.svelte.js";
  import SessionCard from "./SessionCard.svelte";
  import {
    COLUMN_GAP,
    DASHBOARD_TITLE,
    GITHUB_SETTINGS,
    REFRESH,
    columnCount,
    cardHeight,
    freshness,
    gitLine,
    masonry,
    pullSection,
    refreshDisabled,
  } from "./dashboard.js";

  const TICK = 15000;

  /** @type {{sessions: any[], onOpen: (slug: string) => void, store: {status: import("./dashboard.js").DashboardStatus, refresh: () => void}, onSettings: (tab: "github") => void}} */
  let { sessions = [], onOpen, store, onSettings } = $props();

  let now = $state(Date.now());
  $effect(() => {
    const timer = setInterval(() => (now = Date.now()), TICK);
    return () => clearInterval(timer);
  });

  let status = $derived(store.status);
  let header = $derived(freshness(status, now));
  let disabled = $derived(refreshDisabled(status, now));
  let sections = $derived(
    sessions.map((row) => pullSection(status, row.slug, now, { expanded: expanded.has(row.slug) })),
  );

  let width = $state(0);
  let columns = $derived(
    masonry(
      sections.map(cardHeight),
      columnCount(width, uiScale() / 100),
    ),
  );
</script>

<section class="dashboard" aria-labelledby="dashboard-title">
  <!-- Measured here rather than on the grid, whose height changes with the count it is measured for. -->
  <header bind:clientWidth={width}>
    <div></div>
    <div class="heading">
      <h1 id="dashboard-title">{DASHBOARD_TITLE}</h1>
      {#if header.text}
        <p class="freshness" class:warn={header.warn}>
          <span>{header.text}</span>{#if header.settings}<Button variant="ghost" size="sm" onclick={() => onSettings("github")}
              >{GITHUB_SETTINGS}</Button
            >{/if}
        </p>
      {/if}
    </div>
    <div class="actions" class:spinning={status.refreshing}>
      <Button variant="secondary" icon="refresh" {disabled} aria-busy={status.refreshing} onclick={() => store.refresh()}>{REFRESH}</Button>
    </div>
  </header>

  <div class="grid" style:--gap="{COLUMN_GAP}px">
    {#each columns as column, c (c)}
      <div class="column">
        {#each column as index (sessions[index].slug)}
          <SessionCard
            row={sessions[index]}
            position={index}
            {onOpen}
            section={sections[index]}
            git={gitLine(status.sessions?.[sessions[index].slug]?.repos)}
            {now}
            onExpand={(slug) => expanded.add(slug)} />
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
    align-items: start;
  }

  .heading {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: calc(6px * var(--ui-scale));
    text-align: center;
  }

  h1 {
    margin: 0;
    font: var(--display-md);
    letter-spacing: var(--display-tracking);
    color: var(--text-primary);
  }

  .freshness {
    margin: 0;
    display: flex;
    align-items: center;
    gap: calc(8px * var(--ui-scale));
    font: var(--machine-xs);
    color: var(--text-muted);
  }

  .freshness.warn {
    color: var(--state-waiting);
  }

  .actions {
    justify-self: end;
  }

  .spinning :global(.icon) {
    animation: spin 1s linear infinite;
  }

  @keyframes spin {
    to {
      transform: rotate(360deg);
    }
  }

  @media (prefers-reduced-motion: reduce) {
    .spinning :global(.icon) {
      animation: none;
    }
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
