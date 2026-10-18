<script>
  import CapsLabel from "../core/CapsLabel.svelte";
  import Checkbox from "../core/Checkbox.svelte";
  import SegmentedControl from "../forms/SegmentedControl.svelte";
  import TextField from "../forms/TextField.svelte";
  import { macOS } from "../platform.js";
  import Legend from "./Legend.svelte";
  import { EDITOR_PRESETS, matchingPreset } from "./presets.js";
  import TabHeader from "./TabHeader.svelte";

  /** @type {{panel: ReturnType<typeof import("./settings.svelte.js").settings>, labelledby?: string}} */
  let { panel, labelledby } = $props();

  const modifier = macOS ? "⌘" : "Ctrl";
  const placeholders = [
    { term: "{path}", meaning: "Absolute file path. Required." },
    { term: "{line}", meaning: "Line to jump to. 1 if there is none." },
  ];

  let form = $derived(panel.form);
  let preset = $derived(matchingPreset(form.editor));
  let steps = $derived(form.uiScaleSteps.includes(form.uiScale) ? form.uiScaleSteps : [...form.uiScaleSteps, form.uiScale]);
</script>

<TabHeader title="General" id={labelledby}>
  How qrouton opens files, how big it draws, and how it gets your attention.
</TabHeader>

<div class="split">
  <div class="editor">
    <TextField
      label="Editor command"
      aria-label="Editor command"
      bind:value={form.editor}
      valueVoice="literal"
      help={panel.fields.editor ??
        "Runs when you open a file from a plan, spec or diff. Workbenches opened after a change pick it up."}
      helpTone={panel.fields.editor ? "failed" : "muted"} />
    <div class="presets" role="group" aria-label="Start from">
      <span class="lead">Start from</span>
      {#each EDITOR_PRESETS as item (item.name)}
        <button
          type="button"
          class="preset"
          class:selected={preset === item.name}
          aria-pressed={preset === item.name}
          onclick={() => (form.editor = item.template)}>{item.template}</button>
      {/each}
    </div>
  </div>
  <Legend title="Placeholders" rows={placeholders} />
</div>

<div class="scale" role="group" aria-labelledby="ui-scale-label">
  <CapsLabel id="ui-scale-label">UI scale</CapsLabel>
  <SegmentedControl
    segments={steps.map((step) => ({ key: String(step), label: `${step}%` }))}
    value={String(form.uiScale)}
    onSelect={(key) => (form.uiScale = Number(key))} />
  <span class="hint" class:failed={panel.fields.uiScale}>
    {panel.fields.uiScale ?? `${modifier}+ and ${modifier}− step through these too`}
  </span>
</div>

<div class="chime">
  <Checkbox bind:checked={form.chime}>Chime when an agent is waiting for you</Checkbox>
  <span class="hint">Stays quiet for the session you're looking at.</span>
</div>

<style>
  .split {
    display: grid;
    grid-template-columns: minmax(0, 1fr) calc(240px * var(--ui-scale));
    gap: calc(28px * var(--ui-scale));
    align-items: start;
  }

  .editor {
    display: flex;
    flex-direction: column;
    gap: calc(8px * var(--ui-scale));
  }

  .presets {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: calc(6px * var(--ui-scale));
    padding-top: calc(2px * var(--ui-scale));
  }

  .lead {
    margin-right: calc(2px * var(--ui-scale));
    font: var(--machine-xs);
    color: var(--text-faint);
  }

  .preset {
    border: 1px solid var(--border-default);
    border-radius: 0;
    background: transparent;
    padding: calc(5px * var(--ui-scale)) calc(9px * var(--ui-scale));
    font: var(--literal);
    font-size: calc(11px * var(--ui-scale));
    color: var(--text-secondary);
    cursor: pointer;
  }

  .preset:hover {
    background: var(--wash-selected);
  }

  .preset.selected {
    border-color: var(--accent-action);
    color: var(--text-primary);
  }

  .preset:focus-visible {
    outline: none;
    box-shadow: var(--shadow-focus);
  }

  .scale {
    display: flex;
    flex-direction: column;
    align-items: flex-start;
    gap: calc(7px * var(--ui-scale));
  }

  .hint {
    font: var(--machine-sm);
    font-size: calc(11px * var(--ui-scale));
    color: var(--text-muted);
  }

  .hint.failed {
    color: var(--state-failed);
  }

  .chime {
    display: flex;
    flex-direction: column;
    gap: calc(6px * var(--ui-scale));
  }

  .chime > :global(.check) {
    font: var(--machine-md);
    color: var(--text-primary);
  }

  .chime .hint {
    padding-left: calc(24px * var(--ui-scale));
  }
</style>
