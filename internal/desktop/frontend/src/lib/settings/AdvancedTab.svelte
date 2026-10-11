<script>
  import TextField from "../forms/TextField.svelte";
  import Legend from "./Legend.svelte";
  import TabHeader from "./TabHeader.svelte";

  /** @type {{panel: ReturnType<typeof import("./settings.svelte.js").settings>, labelledby?: string}} */
  let { panel, labelledby } = $props();

  const LINEAR_HELP = "Used by Work on issue → Custom script.";

  let form = $derived(panel.form);
  let runners = $derived(form.runners.map((runner) => ({ term: runner.id, meaning: runner.label })));
</script>

<TabHeader title="Advanced" id={labelledby}>Change how agents start and connect other tools to qrouton.</TabHeader>

<div class="split">
  <TextField
    label="Launch overrides"
    aria-label="Launch overrides"
    multiline
    rows="4"
    bind:value={form.launch}
    valueVoice="literal"
    help={panel.fields.launch ??
      "Replaces the command qrouton runs to start an agent. JSON, keyed by runner id, each value the full command as a list. Leave a runner out to keep its default."}
    helpTone={panel.fields.launch ? "failed" : "muted"} />
  {#if runners.length}<Legend title="Runner ids" rows={runners} />{/if}
</div>

<div class="linear">
  <TextField
    label="Linear custom script"
    aria-label="Linear custom script"
    multiline
    rows="6"
    bind:value={form.linear}
    valueVoice="literal"
    help={panel.fields.linear ?? (form.linearPath ? `${LINEAR_HELP} Save writes` : LINEAR_HELP)}
    helpLiteral={panel.fields.linear ? "" : form.linearPath}
    helpTone={panel.fields.linear ? "failed" : "muted"} />
</div>

<style>
  .split {
    display: grid;
    grid-template-columns: minmax(0, 1fr) calc(240px * var(--ui-scale));
    gap: calc(28px * var(--ui-scale));
    align-items: start;
  }

  .linear {
    border-top: 1px solid var(--border-subtle);
    padding-top: calc(18px * var(--ui-scale));
  }
</style>
