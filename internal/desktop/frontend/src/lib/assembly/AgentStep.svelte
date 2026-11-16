<script>
  import { SESSION_MODES } from "../bridge/generated.js";
  import CapsLabel from "../core/CapsLabel.svelte";
  import OptionCard from "../forms/OptionCard.svelte";
  import StepHeading from "../forms/StepHeading.svelte";
  import { badge, statusKey } from "../settings/profiles.js";
  import SignInBadge from "../settings/SignInBadge.svelte";

  import claude from "../assets/agents/claude.png";
  import codex from "../assets/agents/codex.svg";
  import opencode from "../assets/agents/opencode.svg";
  import agy from "../assets/agents/antigravity.svg";

  const LOGOS = { claude, codex, opencode, agy };

  /** @type {{runners?: import("./calls.js").RunnerRow[], runner?: string, profile?: string, statuses?: Record<string, import("../settings/calls.js").ProfileStatus> | null, onManage?: () => void, mode?: import("../bridge/generated.js").SessionMode}} */
  let {
    runners = [],
    runner = $bindable(""),
    profile = $bindable(""),
    statuses = null,
    onManage,
    mode = $bindable(SESSION_MODES.RPI),
  } = $props();

  // A runner with no named profiles reads exactly as it did before profiles existed.
  let profiled = $derived(new Set(runners.filter((row) => row.profile).map((row) => row.id)));

  /** @param {import("./calls.js").RunnerRow} row */
  const title = (row) => (row.profileLabel ? `${row.label} · ${row.profileLabel}` : row.label);
</script>

<StepHeading title="Who runs it, and how?" />

<div class="group">
  {#if profiled.size && onManage}
    <div class="heading">
      <CapsLabel>Agent</CapsLabel>
      <button type="button" class="link" onclick={onManage}>Manage accounts</button>
    </div>
  {:else}
    <CapsLabel>Agent</CapsLabel>
  {/if}
  {#each runners as option (statusKey(option.id, option.profile))}
    {@const status = profiled.has(option.id) ? statuses?.[statusKey(option.id, option.profile)] : undefined}
    <OptionCard
      title={title(option)}
      selected={option.id === runner && option.profile === profile}
      elevated
      onclick={() => {
        runner = option.id;
        profile = option.profile;
      }}>
      {#snippet trailing()}
        {#if status}<SignInBadge shown={badge(status)} />{/if}
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

  .heading {
    display: flex;
    align-items: baseline;
    justify-content: space-between;
  }

  .link {
    border: 0;
    background: none;
    padding: 0;
    font: var(--machine-sm);
    color: var(--text-faint);
    cursor: pointer;
  }

  .link:hover,
  .link:focus-visible {
    color: var(--text-primary);
    outline: none;
  }
</style>
