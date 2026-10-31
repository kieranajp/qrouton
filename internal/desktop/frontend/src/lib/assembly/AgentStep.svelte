<script>
  import { SESSION_MODES } from "../bridge/generated.js";
  import CapsLabel from "../core/CapsLabel.svelte";
  import OptionCard from "../forms/OptionCard.svelte";
  import StepHeading from "../forms/StepHeading.svelte";

  import claude from "../assets/agents/claude.png";
  import codex from "../assets/agents/codex.svg";
  import opencode from "../assets/agents/opencode.svg";
  import agy from "../assets/agents/antigravity.svg";

  const LOGOS = { claude, codex, opencode, agy };

  /** @type {{runners?: {id: string, label: string}[], runner?: string, mode?: import("../bridge/generated.js").SessionMode}} */
  let { runners = [], runner = $bindable(""), mode = $bindable(SESSION_MODES.RPI) } = $props();
</script>

<StepHeading title="Who runs it, and how?" />

<div class="group">
  <CapsLabel>Agent</CapsLabel>
  {#each runners as option (option.id)}
    <OptionCard
      title={option.label}
      selected={option.id === runner}
      elevated
      onclick={() => (runner = option.id)}>
      {#snippet trailing()}
        <img class="logo" class:monochrome={option.id === "opencode"} src={LOGOS[option.id]} alt="" />
      {/snippet}
    </OptionCard>
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
      selected={mode === SESSION_MODES.RPI}
      onclick={() => (mode = SESSION_MODES.RPI)} />
    <OptionCard
      layout="stack"
      title="Open-ended"
      description="Helps directly with no forced workflow. Ask it to switch to guided at any point in the conversation."
      selected={mode === SESSION_MODES.ASSISTANT}
      onclick={() => (mode = SESSION_MODES.ASSISTANT)} />
  </div>
</div>

<style>
  .logo {
    width: calc(28px * var(--ui-scale));
    height: calc(28px * var(--ui-scale));
    object-fit: contain;
    flex: none;
  }

  .monochrome {
    filter: brightness(0) invert(1);
  }

  .group {
    display: flex;
    flex-direction: column;
    gap: calc(10px * var(--ui-scale));
  }

  .modes {
    display: flex;
    gap: calc(12px * var(--ui-scale));
  }
</style>
