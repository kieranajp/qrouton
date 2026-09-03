<script>
  import Dialog from "./Dialog.svelte";
  import RepositoriesStep from "./RepositoriesStep.svelte";
  import { picking } from "./picker.svelte.js";
  import { joining } from "./steps.js";

  /** @type {{slug: string, escalating?: boolean, onClose: () => void}} */
  let { slug, escalating = false, onClose } = $props();

  const picker = picking(() => slug, () => onClose());
  const repos = picker.repos;

  let footer = $derived(picker.status || joining(picker.branch));
</script>

<Dialog
  secondary="Cancel"
  primary={escalating ? "Escalate →" : "Add repositories →"}
  status={footer}
  busy={picker.answering}
  onSecondary={picker.cancel}
  onPrimary={picker.confirm}
  onEscape={picker.cancel}>
  {#if picker.reason}
    <!-- A name the agent asked for that matched nothing in the user's list
         ticks no row below, so this is the only place it is still visible. -->
    <div class="requested">
      <p class="reason">{picker.reason}</p>
      <ul class="rows">
        {#each picker.requested as row (row.id)}
          <li>{row.id} ({row.role}{row.upgrade ? ", upgrade" : ""})</li>
        {/each}
      </ul>
    </div>
  {/if}
  <RepositoriesStep
    bind:query={repos.query}
    orgs={repos.orgs}
    owners={repos.owners}
    failed={repos.failed}
    rows={repos.rows}
    shown={repos.shown}
    total={repos.total}
    tally={repos.tally}
    picks={repos.picks}
    refreshing={repos.refreshing}
    onOwner={repos.owner}
    onRefresh={repos.refetch}
    onRole={repos.role} />
</Dialog>

<style>
  .requested {
    display: flex;
    flex-direction: column;
    gap: 6px;
    padding: 10px 12px;
    border: 1px solid var(--state-guided);
    font: var(--machine-sm);
    color: var(--text-secondary);
  }

  .reason {
    margin: 0;
  }

  .rows {
    margin: 0;
    padding: 0;
    list-style: none;
    display: flex;
    flex-wrap: wrap;
    gap: 4px 14px;
    font: var(--machine-xs);
    color: var(--text-muted);
  }
</style>
