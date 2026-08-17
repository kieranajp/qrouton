<script>
  import AssemblyStep from "../session/AssemblyStep.svelte";
  import AgentStep from "./AgentStep.svelte";
  import DescribeStep from "./DescribeStep.svelte";
  import Dialog from "./Dialog.svelte";
  import RepositoriesStep from "./RepositoriesStep.svelte";
  import { assembling } from "./draft.svelte.js";
  import { destination, labels, last, primary } from "./steps.js";

  /** @type {{onClose: () => void, gated?: boolean}} */
  let { onClose, gated = false } = $props();

  const wizard = assembling(() => onClose());
  const repos = wizard.repos;

  let picked = $derived(repos.tally.editing + repos.tally.reference);
  let footer = $derived(
    wizard.status || (wizard.step === last ? destination(wizard.branch, picked) : ""),
  );
  // Cancelling on step 0 of a gated overlay lands the user on a populated rail
  // over an empty middle, so there is nothing to cancel to. Back on the later
  // steps is unaffected: a user who can go back is not at a dead end.
  let trapped = $derived(gated && wizard.step === 0);
</script>

<Dialog
  steps={labels}
  active={wizard.step}
  secondary={trapped ? undefined : wizard.step ? "← Back" : "Cancel"}
  primary={primary(wizard.step)}
  status={footer}
  busy={wizard.creating}
  onSecondary={() => (wizard.step ? wizard.back() : onClose())}
  onPrimary={wizard.next}
  onEscape={trapped ? undefined : onClose}>
  {#if wizard.creating}
    <div class="progress">
      {#each wizard.progress as row, i (i)}
        <AssemblyStep
          state={row.state}
          label={row.label}
          detail={row.detail}
          percent={row.percent} />
      {/each}
    </div>
  {:else if wizard.step === 0}
    <DescribeStep
      bind:name={wizard.form.name}
      bind:description={wizard.form.description}
      bind:ticket={wizard.form.ticket}
      bind:prefix={wizard.form.prefix}
      prefixes={wizard.prefixes}
      branch={wizard.branch}
      fetching={wizard.fetching}
      hint={wizard.hint}
      onFetch={wizard.load} />
  {:else if wizard.step === 1}
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
  {:else}
    <AgentStep
      runners={wizard.runners}
      bind:runner={wizard.form.runner}
      bind:mode={wizard.form.mode} />
  {/if}
</Dialog>

<style>
  .progress {
    display: flex;
    flex-direction: column;
  }
</style>
