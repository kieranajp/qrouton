<script>
  import { onMount } from "svelte";
  import Dialog from "../assembly/Dialog.svelte";
  import CapsLabel from "../core/CapsLabel.svelte";
  import Checkbox from "../core/Checkbox.svelte";
  import { PLACEMENT_RESULTS } from "../bridge/generated.js";
  import { close, move, report } from "./calls.js";
  import { folderName } from "./folders.js";
  import { reasonLabel } from "./reasons.js";

  /** @type {import("./calls.js").PlacementView | null} */
  let view = $state(null);
  let ticked = $state(/** @type {Record<string, boolean>} */ ({}));
  let outcomes = $state(/** @type {Record<string, import("./calls.js").PlacementOutcome>} */ ({}));
  let busy = $state(false);
  let status = $state("");

  let rows = $derived(view?.misplaced ?? []);
  let chosen = $derived(rows.filter((row) => ticked[row.slug] && !outcomes[row.slug]));
  let moving = $derived(rows.length > 0);
  let sections = $derived(
    [rows.length, view?.pending.length ?? 0, view?.skipped.length ?? 0].filter(Boolean).length,
  );

  onMount(async () => {
    try {
      view = await report();
      ticked = Object.fromEntries((view?.misplaced ?? []).map((row) => [row.slug, true]));
    } catch (error) {
      status = String(error?.message ?? error);
    }
  });

  async function run() {
    busy = true;
    status = "";
    try {
      const done = await move(chosen.map((row) => row.slug));
      outcomes = { ...outcomes, ...Object.fromEntries(done.map((o) => [o.slug, o])) };
    } catch (error) {
      status = String(error?.message ?? error);
    }
    busy = false;
  }

  async function dismiss() {
    busy = true;
    try {
      await close();
    } catch (error) {
      status = String(error?.message ?? error);
      busy = false;
    }
  }

  /** @param {import("./calls.js").PlacementOutcome} outcome */
  const outcomeText = (outcome) =>
    outcome.result === PLACEMENT_RESULTS.MOVED ? "Moved" : `${sentence(outcome.result)}: ${outcome.detail ?? ""}`;
  /** @param {string} text */
  const sentence = (text) => text.charAt(0).toUpperCase() + text.slice(1);
</script>

<Dialog
  compact
  secondary={moving ? "Close" : undefined}
  primary={moving ? "Move" : "Close"}
  {status}
  {busy}
  canAdvance={moving ? chosen.length > 0 : true}
  onSecondary={dismiss}
  onPrimary={moving ? run : dismiss}
  onEscape={dismiss}>
  <h1>Some sessions are in the wrong thoughts folder</h1>

  <p class="lead">
    qrouton keeps a session's thoughts in the folder for its repos' organisation.
    {#if moving}Move puts these where they belong.{:else}These need your attention.{/if}
  </p>

  {#if view?.relaunchNote}
    <p class="note">{view.relaunchNote}</p>
  {/if}

  {#if moving}
    {#if sections > 1}<CapsLabel tone="dim">Move these</CapsLabel>{/if}
    <ul class="rows">
      {#each rows as row (row.slug)}
        <li>
          <Checkbox bind:checked={ticked[row.slug]} disabled={busy || Boolean(outcomes[row.slug])}>
            <span class="name">{row.name}</span>
          </Checkbox>
          <span class="route">
            now in <span title={row.from.path}>{folderName(row.from.path)}</span> · belongs in
            <span title={row.to.path}>{folderName(row.to.path)}</span>
          </span>
          {#if outcomes[row.slug]}
            <span class="outcome" class:failed={outcomes[row.slug].result !== PLACEMENT_RESULTS.MOVED}>
              {outcomeText(outcomes[row.slug])}
            </span>
          {/if}
        </li>
      {/each}
    </ul>
    {#if view?.leavingNote}
      <p class="note">{view.leavingNote}</p>
    {/if}
  {:else if view && !view.skipped.length && !view.pending.length}
    <p class="note">Every session is where it belongs.</p>
  {/if}

  {#if view?.pending.length}
    {#if sections > 1}<CapsLabel tone="dim">Needs attention</CapsLabel>{/if}
    <ul class="rows">
      {#each view.pending as row (row.slug)}
        <li>
          <span class="name">{row.name}</span>
          <span class="route failed">{row.detail}</span>
        </li>
      {/each}
    </ul>
  {/if}

  {#if view?.skipped.length}
    {#if sections > 1}<CapsLabel tone="dim">Left where they are</CapsLabel>{/if}
    <ul class="rows">
      {#each view.skipped as row (row.slug)}
        <li>
          <span class="name">{row.name}</span>
          <span class="route">{reasonLabel(row)}</span>
        </li>
      {/each}
    </ul>
  {/if}
</Dialog>

<style>
  h1 {
    margin: 0;
    font: var(--display-md);
    letter-spacing: var(--display-tracking);
    color: var(--text-primary);
  }

  p {
    margin: 0;
    max-width: 78ch;
    text-wrap: pretty;
  }

  .lead {
    font: var(--machine-md);
    color: var(--text-secondary);
  }

  .note {
    font: var(--machine-sm);
    color: var(--text-muted);
  }

  .rows {
    margin: 0;
    padding: 0;
    list-style: none;
    border-top: 1px solid var(--border-subtle);
  }

  li {
    display: flex;
    align-items: baseline;
    gap: calc(16px * var(--ui-scale));
    padding: calc(9px * var(--ui-scale)) 0;
    border-bottom: 1px solid var(--border-subtle);
  }

  .name {
    font: var(--machine-md);
    color: var(--text-primary);
  }

  .route {
    font: var(--machine-sm);
    color: var(--text-muted);
    text-wrap: pretty;
  }

  .route.failed {
    color: var(--state-failed);
  }

  .outcome {
    margin-left: auto;
    font: var(--machine-sm);
    color: var(--state-success);
  }

  .outcome.failed {
    color: var(--state-failed);
  }
</style>
