<script>
  import Button from "../core/Button.svelte";
  import Chip from "../core/Chip.svelte";
  import StepHeading from "../forms/StepHeading.svelte";
  import TextField from "../forms/TextField.svelte";

  const LINEAR_HELP = "Used by Work on issue → Custom script.";

  /** @type {{orgs?: string[], orgInput?: string, root?: string, editor?: string, launch?: string, linear?: string, linearPath?: string, stickerLabels?: {star: string, bookmark: string, question: string, exclamation: string}, fields?: Record<string, string>, restartRequired?: boolean, onAddOrg?: () => void, onRemoveOrg?: (org: string) => void, onQuit?: () => void}} */
  let {
    orgs = [],
    orgInput = $bindable(""),
    root = $bindable(""),
    editor = $bindable(""),
    launch = $bindable(""),
    linear = $bindable(""),
    linearPath = "",
    stickerLabels = $bindable({ star: "", bookmark: "", question: "", exclamation: "" }),
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

<TextField
  label="Editor"
  bind:value={editor}
  valueVoice="literal"
  help={fields.editor ?? "One {} placeholder for the file path"}
  helpTone={fields.editor ? "failed" : "muted"} />

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
    gap: 8px;
  }

  .org {
    display: inline-flex;
    align-items: center;
    gap: 4px;
  }

  .sticker-labels {
    margin: 0;
    border: 0;
    padding: 0;
  }

  .sticker-labels legend {
    margin-bottom: 10px;
    padding: 0;
    font: var(--display-sm);
    color: var(--text-primary);
  }

  .sticker-fields {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 14px 18px;
  }

  .banner {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 14px;
    padding: 10px 12px;
    background: var(--surface-chrome);
    border: 1px solid var(--border-subtle);
    font: var(--machine-sm);
    color: var(--text-secondary);
  }
</style>
