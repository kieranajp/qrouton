<script>
  import CapsLabel from "../core/CapsLabel.svelte";
  import OptionCard from "../forms/OptionCard.svelte";
  import StepHeading from "../forms/StepHeading.svelte";

  // qrouton wires MCP and hooks per runner, so the set is closed and so is the
  // copy for it.
  const ABOUT = {
    claude: "Subagents, skills and the window tools. What guided mode is tuned against.",
    codex: "Works in both modes. Needs depth ≥ 2 to nest subagents when guided.",
    opencode: "Open-ended sessions. No subagents, so guided mode runs flat.",
  };

  /** @type {{runners?: {id: string, label: string}[], runner?: string, mode?: string}} */
  let { runners = [], runner = $bindable(""), mode = $bindable("rpi") } = $props();
</script>

<StepHeading title="Who runs it, and how?">
  Only agents found on your PATH are listed. Both choices can be changed later.
</StepHeading>

<div class="group">
  <CapsLabel>Agent</CapsLabel>
  {#each runners as option (option.id)}
    <OptionCard
      title={option.label}
      description={ABOUT[option.id]}
      meta={option.id}
      selected={option.id === runner}
      elevated
      onclick={() => (runner = option.id)} />
  {/each}
</div>

<div class="group">
  <CapsLabel>How it works</CapsLabel>
  <div class="modes">
    <OptionCard
      layout="stack"
      title="Guided (RPI)"
      description="Researches the codebase, writes a plan you can read, then implements it phase by phase with test gates between them."
      accent="var(--state-guided)"
      wash="var(--wash-guided)"
      selected={mode === "rpi"}
      onclick={() => (mode = "rpi")} />
    <OptionCard
      layout="stack"
      title="Open-ended"
      description="Helps directly with no forced workflow. Ask it to switch to guided at any point in the conversation."
      selected={mode === "assistant"}
      onclick={() => (mode = "assistant")} />
  </div>
</div>

<style>
  .group {
    display: flex;
    flex-direction: column;
    gap: 10px;
  }

  .modes {
    display: flex;
    gap: 12px;
  }
</style>
