<script>
  import Button from "../core/Button.svelte";
  import CapsLabel from "../core/CapsLabel.svelte";
  import Chip from "../core/Chip.svelte";
  import StepHeading from "../forms/StepHeading.svelte";
  import TextField from "../forms/TextField.svelte";
  import OllamaStatus from "../OllamaStatus.svelte";
  import ThoughtsSettings from "./ThoughtsSettings.svelte";

  const LINEAR_HELP = "Used by Work on issue → Custom script.";

  /** @type {{orgs?: string[], orgInput?: string, root?: string, editor?: string, launch?: string, linear?: string, linearPath?: string, stickerLabels?: {star: string, bookmark: string, question: string, exclamation: string}, chime?: boolean, uiScale?: number, uiScaleSteps?: number[], thoughts?: import("./thoughts.js").ThoughtsForm, loadedThoughts?: import("./thoughts.js").ThoughtsForm & {derived?: string}, onAddFolder?: () => void, onRemoveFolder?: (index: number) => void, fields?: Partial<Record<import("../bridge/generated.js").SettingsField, string>>, restartRequired?: boolean, onAddOrg?: () => void, onRemoveOrg?: (org: string) => void, onQuit?: () => void}} */
  let {
    orgs = [],
    orgInput = $bindable(""),
    root = $bindable(""),
    editor = $bindable(""),
    launch = $bindable(""),
    linear = $bindable(""),
    linearPath = "",
    stickerLabels = $bindable({ star: "", bookmark: "", question: "", exclamation: "" }),
    chime = $bindable(true),
    uiScale = $bindable(100),
    uiScaleSteps = [],
    thoughts = $bindable({ default: "", roots: [] }),
    loadedThoughts = { default: "", roots: [], derived: "" },
    onAddFolder,
    onRemoveFolder,
    fields = {},
    restartRequired = false,
    onAddOrg,
    onRemoveOrg,
    onQuit,
  } = $props();
</script>

<StepHeading title="Settings">Written to config.json.</StepHeading>

<TextField label="GitHub orgs" bind:value={orgInput} placeholder="org-name">
  {#snippet trailing()}
    <Button variant="secondary" onclick={onAddOrg}>Add</Button>
  {/snippet}
</TextField>
{#if orgs.length}
  <div class="orgs">
    {#each orgs as org (org)}
      <span class="org">
        <Chip>{org}</Chip>
        <Button variant="ghost" size="sm" aria-label="Remove {org}" onclick={() => onRemoveOrg?.(org)}
          >×</Button>
      </span>
    {/each}
  </div>
{/if}

<TextField
  label="Sessions root"
  bind:value={root}
  help={fields.root ?? "Takes effect for sessions started after a restart"}
  helpTone={fields.root ? "failed" : "muted"} />

<ThoughtsSettings
  bind:thoughts
  loaded={loadedThoughts}
  error={fields.thoughts ?? ""}
  onAdd={onAddFolder}
  onRemove={onRemoveFolder} />

<div class="semantic">
  <CapsLabel>Semantic search</CapsLabel>
  <OllamaStatus />
</div>

<TextField
  label="Editor"
  bind:value={editor}
  valueVoice="literal"
  help={fields.editor ?? "One {} placeholder for the file path"}
  helpTone={fields.editor ? "failed" : "muted"} />

<label class="chime">
  <input type="checkbox" bind:checked={chime} />
  Chime when an agent is waiting for you
</label>

<label class="scale">
  <CapsLabel>UI scale</CapsLabel>
  <select bind:value={uiScale}>
    {#each uiScaleSteps.includes(uiScale) ? uiScaleSteps : [...uiScaleSteps, uiScale] as step (step)}
      <option value={step}>{step}%</option>
    {/each}
  </select>
  {#if fields.uiScale}<span class="help">{fields.uiScale}</span>{/if}
</label>

<fieldset class="sticker-labels">
  <legend>Session stickers</legend>
  <div class="sticker-fields">
    <TextField
      label="Blue star"
      aria-label="Blue star meaning"
      bind:value={stickerLabels.star}
      help={fields.star}
      helpTone={fields.star ? "failed" : "muted"} />
    <TextField
      label="Green bookmark"
      aria-label="Green bookmark meaning"
      bind:value={stickerLabels.bookmark}
      help={fields.bookmark}
      helpTone={fields.bookmark ? "failed" : "muted"} />
    <TextField
      label="Orange question mark"
      aria-label="Orange question mark meaning"
      bind:value={stickerLabels.question}
      help={fields.question}
      helpTone={fields.question ? "failed" : "muted"} />
    <TextField
      label="Red exclamation mark"
      aria-label="Red exclamation mark meaning"
      bind:value={stickerLabels.exclamation}
      help={fields.exclamation}
      helpTone={fields.exclamation ? "failed" : "muted"} />
  </div>
</fieldset>

<TextField
  label="Launch overrides"
  multiline
  bind:value={launch}
  valueVoice="literal"
  help={fields.launch ?? "JSON, keyed by runner id"}
  helpTone={fields.launch ? "failed" : "muted"} />

<TextField
  label="Linear custom script"
  multiline
  rows="6"
  bind:value={linear}
  valueVoice="literal"
  help={fields.linear ?? (linearPath ? `${LINEAR_HELP} Save writes` : LINEAR_HELP)}
  helpLiteral={fields.linear ? "" : linearPath}
  helpTone={fields.linear ? "failed" : "muted"} />

{#if restartRequired}
  <div class="banner">
    <span>Quit qrouton to use the new sessions root</span>
    <Button variant="secondary" onclick={onQuit}>Quit qrouton</Button>
  </div>
{/if}

<style>
  .orgs {
    display: flex;
    flex-wrap: wrap;
    gap: calc(8px * var(--ui-scale));
  }

  .org {
    display: inline-flex;
    align-items: center;
    gap: calc(4px * var(--ui-scale));
  }

  .semantic {
    display: flex;
    flex-direction: column;
    gap: 7px;
  }

  .sticker-labels {
    margin: 0;
    border: 0;
    padding: 0;
  }

  .sticker-labels legend {
    margin-bottom: calc(10px * var(--ui-scale));
    padding: 0;
    font: var(--display-sm);
    color: var(--text-primary);
  }

  .sticker-fields {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: calc(14px * var(--ui-scale)) calc(18px * var(--ui-scale));
  }

  .chime {
    display: inline-flex;
    align-items: center;
    gap: calc(8px * var(--ui-scale));
    font: var(--machine-md);
    color: var(--text-primary);
    cursor: pointer;
  }

  .chime input {
    appearance: none;
    width: calc(13px * var(--ui-scale));
    height: calc(13px * var(--ui-scale));
    margin: 0;
    border: var(--border-width) solid var(--border-default);
    background: transparent;
    cursor: pointer;
  }

  .chime input:checked {
    border: none;
    background: var(--accent-action);
  }

  .chime input:focus-visible {
    outline: none;
    box-shadow: var(--shadow-focus);
  }

  .scale {
    display: flex;
    flex-direction: column;
    align-items: flex-start;
    gap: var(--space-2);
  }

  .scale select {
    border: 1px solid var(--border-default);
    background: var(--surface-chrome);
    padding: var(--space-3) var(--space-5);
    font: var(--machine-md);
    color: var(--text-primary);
  }

  .scale select:focus-visible {
    outline: none;
    border-color: var(--accent-action);
    box-shadow: var(--shadow-focus);
  }

  .scale .help {
    font: var(--machine-sm);
    color: var(--state-failed);
  }

  .banner {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: calc(14px * var(--ui-scale));
    padding: calc(10px * var(--ui-scale)) calc(12px * var(--ui-scale));
    background: var(--surface-chrome);
    border: 1px solid var(--border-subtle);
    font: var(--machine-sm);
    color: var(--text-secondary);
  }
</style>
