<script>
  import Dialog from "../assembly/Dialog.svelte";
  import CapsLabel from "../core/CapsLabel.svelte";
  import OrgsScreen from "./OrgsScreen.svelte";
  import PanelsScreen from "./PanelsScreen.svelte";
  import RootScreen from "./RootScreen.svelte";
  import SessionScreen from "./SessionScreen.svelte";
  import Welcome from "./Welcome.svelte";
  import { firstRun } from "./firstrun.svelte.js";
  import { back, blocking, caps, pip, primary, title, total } from "./screens.js";

  const flow = firstRun();

  let blocked = $derived(blocking(flow.step, flow.form.orgs, flow.orgInput));
</script>

<Dialog
  pips={total}
  active={pip(flow.step)}
  secondary={back(flow.step) || undefined}
  primary={primary(flow.step)}
  status={blocked || flow.status}
  busy={flow.busy}
  canAdvance={!blocked}
  onSecondary={flow.back}
  onPrimary={flow.next}>
  <div class="title">{title}</div>

  {#if caps(flow.step)}
    <CapsLabel>{caps(flow.step)}</CapsLabel>
  {/if}

  {#if flow.step === 0}
    <Welcome />
  {:else if flow.step === 1}
    <SessionScreen />
  {:else if flow.step === 2}
    <PanelsScreen />
  {:else if flow.step === 3}
    <OrgsScreen
      orgs={flow.form.orgs}
      bind:orgInput={flow.orgInput}
      login={flow.login}
      onAddOrg={flow.add}
      onRemoveOrg={flow.remove} />
  {:else}
    <RootScreen bind:root={flow.form.root} error={flow.fields.root ?? ""} onChoose={flow.choose} />
  {/if}
</Dialog>

<style>
  .title {
    margin: -4px 0 4px;
    padding-bottom: 14px;
    border-bottom: 1px solid var(--border-subtle);
    font: var(--machine-sm);
    color: var(--text-faint);
  }
</style>
