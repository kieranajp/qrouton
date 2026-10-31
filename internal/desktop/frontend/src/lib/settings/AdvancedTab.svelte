<script>
  import Button from "../core/Button.svelte";
  import CapsLabel from "../core/CapsLabel.svelte";
  import TextField from "../forms/TextField.svelte";
  import { flash } from "../flash.svelte.js";
  import { copyText } from "../wails.js";
  import Legend from "./Legend.svelte";
  import TabHeader from "./TabHeader.svelte";

  /** @type {{panel: ReturnType<typeof import("./settings.svelte.js").settings>, labelledby?: string}} */
  let { panel, labelledby } = $props();

  let form = $derived(panel.form);
  let linear = $derived(panel.linear);
  let runners = $derived(form.runners.map((runner) => ({ term: runner.id, meaning: runner.label })));
  let reviewerRunner = $derived(panel.reviewerRunners.find((runner) => runner.id === form.reviewer?.harness));
  let reasoningValues = $derived(reviewerRunner?.reviewer?.reasoningValues ?? []);
  let unsupportedReasoning = $derived(
    !!form.reviewer?.reasoning &&
      !reviewerRunner?.reviewer?.reasoningEditable &&
      !reasoningValues.includes(form.reviewer.reasoning),
  );
  /** @type {ReturnType<typeof flash<boolean>>} */
  const copied = flash();

  async function copySnippet() {
    try {
      await copyText(linear.snippet);
      copied.show(true);
    } catch {
      copied.clear();
    }
  }
</script>

<TabHeader title="Advanced" id={labelledby}>
  Change how agents start and connect other tools to qrouton.
  {#snippet aside()}
    <div class="open-config">
      <Button variant="cube" icon="document" onclick={panel.openConfig}>Open config.json</Button>
      <span class:failed={panel.actionError}>
        {panel.actionError || "Opens in your default text editor. Every setting lives here."}
      </span>
    </div>
  {/snippet}
</TabHeader>

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

<section class="reviewer" aria-labelledby="reviewer-label">
  <CapsLabel id="reviewer-label">Independent code review</CapsLabel>
  <label>
    <CapsLabel>Reviewer harness</CapsLabel>
    <select
      value={form.reviewer ? form.reviewer.harness || "__choose__" : ""}
      onchange={(event) => panel.selectReviewer(event.currentTarget.value)}>
      <option value="">None</option>
      {#if form.reviewer && !form.reviewer.harness}<option value="__choose__" disabled>Choose an installed harness</option>{/if}
      {#if form.reviewer?.harness && !reviewerRunner}
        <option value={form.reviewer.harness} disabled>{form.reviewer.harness} (unavailable)</option>
      {/if}
      {#each panel.reviewerRunners as runner (runner.id)}
        <option value={runner.id}>{runner.label}</option>
      {/each}
    </select>
  </label>
  {#if !form.reviewer}
    <p>None uses standard non-adversarial review in the orchestrator’s current harness, without a second reviewer.</p>
    <Button disabled={!panel.runnersReady} onclick={panel.configureReviewer}>Configure reviewer</Button>
  {:else}
    <TextField
      label="Reviewer model"
      aria-label="Reviewer model"
      bind:value={form.reviewer.model}
      valueVoice="literal"
      help={reviewerRunner?.reviewer?.modelHelp} />
    {#if reviewerRunner?.reviewer?.reasoningEditable}
      <TextField
        label="Reviewer reasoning"
        aria-label="Reviewer reasoning"
        bind:value={form.reviewer.reasoning}
        valueVoice="literal"
        help={reviewerRunner.reviewer.reasoningHelp} />
    {:else}
      <label>
        <CapsLabel>Reviewer reasoning</CapsLabel>
        <select bind:value={form.reviewer.reasoning}>
          <option value="">Provider default</option>
          {#if unsupportedReasoning}<option value={form.reviewer.reasoning} disabled>{form.reviewer.reasoning} (unsupported)</option>{/if}
          {#each reasoningValues as value (value)}<option {value}>{value}</option>{/each}
        </select>
        <span class:failed={unsupportedReasoning}>{reviewerRunner?.reviewer?.reasoningHelp}</span>
      </label>
    {/if}
    <p>Saving enables independent adversarial review availability. It does not start a review.</p>
  {/if}
  {#if panel.reviewerError}<p class="failed">{panel.reviewerError}</p>{/if}
</section>

<section class="linear" aria-labelledby="linear-label">
  <CapsLabel id="linear-label">Linear</CapsLabel>
  <p>
    Adds qrouton as a custom script under Work on issue in Linear's desktop app. Choosing it starts a session for
    that issue.
  </p>
  {#if linear.snippet}
    <div class="install">
      <Button disabled={panel.installing} onclick={panel.installLinear}>
        {linear.replaces ? "Install Linear integration, replacing the current entry" : "Install Linear integration"}
      </Button>
      {#if panel.installed && !linear.error}
        <span class="done">Installed in {linear.path}</span>
      {:else if linear.replaces}
        <span class="note">{linear.path} already has a different openIssue entry. Install replaces only that one.</span>
      {/if}
    </div>
    <p>Or add this entry to <code>{linear.path}</code> yourself, keeping any other keys in it.</p>
    <div class="snippet">
      <button type="button" class="copy" aria-label="Copy the Linear entry" onclick={copySnippet}>
        {copied.value ? "Copied" : "Copy"}
      </button>
      <pre aria-label="Linear entry">{linear.snippet}</pre>
    </div>
  {/if}
  {#if linear.error}<p class="failed">{linear.error}</p>{/if}
</section>

<style>
  .open-config {
    flex: none;
    display: flex;
    flex-direction: column;
    align-items: flex-end;
    gap: calc(6px * var(--ui-scale));
    font: var(--machine-xs);
    color: var(--text-faint);
  }

  .split {
    display: grid;
    grid-template-columns: minmax(0, 1fr) calc(240px * var(--ui-scale));
    gap: calc(28px * var(--ui-scale));
    align-items: start;
  }

  .linear {
    display: flex;
    flex-direction: column;
    gap: calc(10px * var(--ui-scale));
    border-top: 1px solid var(--border-subtle);
    padding-top: calc(18px * var(--ui-scale));
  }

  .reviewer {
    display: flex;
    flex-direction: column;
    gap: calc(10px * var(--ui-scale));
    border-top: 1px solid var(--border-subtle);
    padding-top: calc(18px * var(--ui-scale));
  }

  .reviewer label {
    display: flex;
    flex-direction: column;
    gap: calc(7px * var(--ui-scale));
  }

  .reviewer select {
    width: 100%;
  }

  .reviewer p,
  .reviewer span {
    margin: 0;
    font: var(--machine-sm);
    color: var(--text-muted);
  }

  .reviewer .failed {
    color: var(--state-failed);
  }

  p {
    margin: 0;
    font: var(--machine-sm);
    font-size: calc(11px * var(--ui-scale));
    color: var(--text-muted);
    text-wrap: pretty;
  }

  code {
    font: var(--literal);
    font-size: calc(11px * var(--ui-scale));
    color: var(--text-primary);
  }

  .install {
    display: flex;
    align-items: center;
    gap: calc(14px * var(--ui-scale));
  }

  .done,
  .note {
    font: var(--machine-xs);
  }

  .done {
    color: var(--state-success);
  }

  .note {
    color: var(--state-waiting);
  }

  .failed,
  .open-config .failed {
    color: var(--state-failed);
  }

  .snippet {
    position: relative;
    border: 1px solid var(--border-subtle);
    background: var(--surface-chrome);
  }

  pre {
    margin: 0;
    padding: calc(10px * var(--ui-scale)) calc(12px * var(--ui-scale));
    overflow-x: auto;
    font: var(--literal);
    font-size: calc(11px * var(--ui-scale));
    color: var(--text-secondary);
  }

  .copy {
    position: absolute;
    top: calc(8px * var(--ui-scale));
    right: calc(12px * var(--ui-scale));
    border: 0;
    background: none;
    padding: 0;
    font: var(--machine-xs);
    color: var(--text-faint);
    cursor: pointer;
  }

  .copy:hover,
  .copy:focus-visible {
    color: var(--text-primary);
    outline: none;
  }
</style>
