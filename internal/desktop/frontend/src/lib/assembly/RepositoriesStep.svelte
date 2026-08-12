<script>
  import Button from "../core/Button.svelte";
  import CapsLabel from "../core/CapsLabel.svelte";
  import Chip from "../core/Chip.svelte";
  import SegmentedControl from "../forms/SegmentedControl.svelte";
  import TextField from "../forms/TextField.svelte";
  import RepoRow from "../session/RepoRow.svelte";

  /** @type {{query?: string, orgs?: string[], owners?: string[], failed?: string[], rows?: {id: string, meta: string, role: 'off'|'editing'|'reference', locked: boolean}[], shown?: number, total?: number, tally?: {editing: number, reference: number}, picks?: {id: string, role: string, glyph: string, meta: string}[], refreshing?: boolean, onOwner?: (org: string) => void, onRefresh?: () => void, onRole?: (id: string, role: string) => void}} */
  let {
    query = $bindable(""),
    orgs = [],
    owners = [],
    failed = [],
    rows = [],
    shown = 0,
    total = 0,
    tally = { editing: 0, reference: 0 },
    picks = [],
    refreshing = false,
    onOwner,
    onRefresh,
    onRole,
  } = $props();

  let segments = $derived(
    orgs.map((org) => ({
      key: org,
      label: org,
      accent: failed.includes(org) ? "var(--state-failed)" : "var(--accent-action)",
    })),
  );
</script>

<div class="heading">
  <span class="title">Which repositories?</span>
  <span class="helper">
    <span class="editing">Editing</span> means the agent may change it, on a new branch.
    <span class="reference">Reference</span> is checked out read-only, for context.
  </span>
</div>

<div class="controls">
  <div class="search">
    <TextField icon="⌕" valueVoice="literal" bind:value={query} />
  </div>
  <SegmentedControl {segments} value={owners} multiple onSelect={onOwner} />
  <Button variant="secondary" onclick={onRefresh} disabled={refreshing}
    >{refreshing ? "Refreshing…" : "↻ Refresh"}</Button>
</div>

<div class="list">
  <div class="tally">
    <CapsLabel tone="dim">{shown} of {total} shown</CapsLabel>
    <span class="roles">{tally.editing} editing · {tally.reference} reference</span>
  </div>
  <div class="rows">
    {#each rows as row (row.id)}
      <RepoRow
        name={row.id}
        meta={row.meta}
        role={row.role}
        locked={row.locked}
        onRoleChange={(role) => onRole?.(row.id, role)} />
    {/each}
  </div>
</div>

<div class="selected">
  <CapsLabel>Selected</CapsLabel>
  <div class="chips">
    {#each picks as pick (pick.id)}
      <Chip tone={pick.role} glyph={pick.glyph} meta={pick.meta}>{pick.id}</Chip>
    {/each}
  </div>
</div>

<style>
  .heading {
    display: flex;
    align-items: flex-end;
    gap: 18px;
  }

  .title {
    font: var(--display-md);
    letter-spacing: var(--display-tracking);
    color: var(--text-primary);
  }

  .helper {
    flex: 1;
    font: var(--machine-sm);
    color: var(--text-muted);
    padding-bottom: 5px;
  }

  .editing {
    color: var(--role-editing);
  }

  .reference {
    color: var(--role-reference);
  }

  .controls {
    display: flex;
    gap: 8px;
  }

  .search {
    flex: 1;
    min-width: 0;
  }

  .list {
    flex: none;
    display: flex;
    flex-direction: column;
    border: 1px solid var(--border-default);
  }

  .tally {
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 8px 12px;
    background: var(--surface-chrome);
    border-bottom: 1px solid var(--border-subtle);
  }

  .roles {
    margin-left: auto;
    font: var(--machine-sm);
    font-size: 11px;
    color: var(--text-muted);
  }

  /* One height whatever the search left behind: the count above says how much
     of the list this is. */
  .rows {
    height: 224px;
    overflow: hidden auto;
  }

  .selected {
    display: flex;
    flex-direction: column;
    gap: 9px;
  }

  .chips {
    display: flex;
    flex-wrap: wrap;
    gap: 8px;
  }
</style>
