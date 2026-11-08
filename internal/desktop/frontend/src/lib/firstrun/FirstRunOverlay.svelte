<script>
  import Dialog from "../assembly/Dialog.svelte";
  import CapsLabel from "../core/CapsLabel.svelte";
  import OllamaScreen from "./OllamaScreen.svelte";
  import OrgsScreen from "./OrgsScreen.svelte";
  import PanelsScreen from "./PanelsScreen.svelte";
  import RootScreen from "./RootScreen.svelte";
  import SessionScreen from "./SessionScreen.svelte";
  import ThoughtsScreen from "./ThoughtsScreen.svelte";
  import Welcome from "./Welcome.svelte";
  import { firstRun } from "./firstrun.svelte.js";
  import { back, blocking, caps, icons, pip, primary, screenKey, title, total } from "./screens.js";

  const flow = firstRun();

  /** @type {Record<import("./screens.js").Screen, import("svelte").Snippet>} */
  const renderers = { welcome, session, panels, orgs, root, thoughts, ollama };

  let blocked = $derived(blocking(flow.step, flow.form.orgs, flow.orgInput));
</script>

{#snippet welcome()}
  <Welcome />
{/snippet}

{#snippet session()}
  <SessionScreen />
{/snippet}

{#snippet panels()}
  <PanelsScreen />
{/snippet}

{#snippet orgs()}
  <OrgsScreen
    orgs={flow.form.orgs}
    bind:orgInput={flow.orgInput}
    login={flow.login}
    onAddOrg={flow.add}
    onRemoveOrg={flow.remove} />
{/snippet}

{#snippet root()}
  <RootScreen bind:root={flow.form.root} error={flow.fields.root ?? ""} onChoose={flow.choose} />
{/snippet}

{#snippet thoughts()}
  <ThoughtsScreen
    bind:thoughts={flow.thoughts}
    derived={flow.derivedThoughts}
    error={flow.fields.thoughts ?? ""}
    onChoose={flow.chooseThoughts} />
{/snippet}

{#snippet ollama()}
  <OllamaScreen />
{/snippet}

<Dialog
  pips={total}
  active={pip(flow.step)}
  secondary={back(flow.step) || undefined}
  secondaryIcon={icons.back}
  primary={primary(flow.step)}
  primaryIcon={icons.primary}
  status={blocked || flow.status}
  busy={flow.busy}
  canAdvance={!blocked}
  onSecondary={flow.back}
  onPrimary={flow.next}>
  <div class="title">{title}</div>

  {#if caps(flow.step)}
    <CapsLabel>{caps(flow.step)}</CapsLabel>
  {/if}

  {@render renderers[screenKey(flow.step)]()}
</Dialog>

<style>
  .title {
    margin: calc(-4px * var(--ui-scale)) 0 calc(4px * var(--ui-scale));
    padding-bottom: calc(14px * var(--ui-scale));
    border-bottom: 1px solid var(--border-subtle);
    font: var(--machine-sm);
    color: var(--text-faint);
  }
</style>
