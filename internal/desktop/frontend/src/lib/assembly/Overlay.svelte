<script>
  import { onMount } from "svelte";
  import AgentStep from "./AgentStep.svelte";
  import DescribeStep from "./DescribeStep.svelte";
  import Dialog from "./Dialog.svelte";
  import ProgressRows from "./ProgressRows.svelte";
  import RepositoriesStep from "./RepositoriesStep.svelte";
  import { assembling } from "./draft.svelte.js";
  import { begin, end } from "./calls.js";
  import { destination, labels, last, primary, stepKey } from "./steps.js";

  /** @type {{onClose: () => void, gated?: boolean, settingsOpen?: boolean, onSettings?: (tab: import("../settings/tabs.js").SettingsTab) => void}} */
  let { onClose, gated = false, settingsOpen = false, onSettings } = $props();

  const wizard = assembling(() => onClose());
  const repos = wizard.repos;
  /** @type {Record<import("./steps.js").Step, import("svelte").Snippet>} */
  const renderers = { describe, repositories, agent };

  let picked = $derived(repos.tally.editing + repos.tally.reference);
  let footer = $derived(
    wizard.status || (wizard.step === last ? destination(wizard.branch, picked) : ""),
  );
  // A gated overlay is all an install with no sessions has, so step 0 has nothing
  // to cancel to. Back on the later steps is unaffected: a user who can go back is
  // not at a dead end.
  let trapped = $derived(gated && wizard.step === 0);
  let owned = $state(false);

  // A profile added or signed in while Settings covered the wizard shows once it closes.
  let covered = false;
  $effect(() => {
    if (covered && !settingsOpen) wizard.readRunners();
    covered = settingsOpen;
  });

  onMount(() => {
    let live = true;
    let generation = 0;
    begin()
      .then((seed) => {
        if (!live) {
          if (seed?.generation) end(seed.generation).catch(() => {});
          return;
        }
        generation = seed?.generation ?? 0;
        wizard.form.entropy = seed?.entropy ?? "";
        if (seed?.ticket) wizard.seed(seed.ticket);
        owned = true;
      })
      .catch(() => {});
    return () => {
      live = false;
      if (generation) end(generation).catch(() => {});
    };
  });
</script>

{#snippet describe()}
  <DescribeStep
    bind:name={wizard.form.name}
    bind:branchDescription={wizard.form.branchDescription}
    bind:description={wizard.form.description}
    bind:ticket={wizard.form.ticket}
    bind:prefix={wizard.form.prefix}
    prefixes={wizard.prefixes}
    branch={wizard.branch}
    fetching={wizard.fetching}
    hint={wizard.hint}
    onFetch={wizard.load} />
{/snippet}

{#snippet repositories()}
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
    onRole={repos.role}
    onBase={repos.base}
    onBaseOpen={repos.loadBranches} />
{/snippet}

{#snippet agent()}
  <AgentStep
    runners={wizard.runners}
    bind:runner={wizard.form.runner}
    bind:profile={wizard.form.profile}
    statuses={wizard.statuses}
    onManage={onSettings ? () => onSettings("agents") : undefined}
    bind:mode={wizard.form.mode} />
{/snippet}

{#if owned}
<Dialog
  steps={labels}
  active={wizard.step}
  secondary={trapped ? undefined : wizard.step ? "Back" : "Cancel"}
  secondaryIcon={wizard.step ? "back" : undefined}
  primary={primary(wizard.step)}
  primaryIcon="forward"
  status={footer}
  busy={wizard.creating}
  onSecondary={() => (wizard.step ? wizard.back() : onClose())}
  onPrimary={wizard.next}
  onEscape={trapped ? undefined : onClose}>
  {#if wizard.creating}
    <ProgressRows rows={wizard.progress} />
  {:else}
    {@render renderers[stepKey(wizard.step)]()}
  {/if}
</Dialog>
{/if}
