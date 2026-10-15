<script>
  import CapsLabel from "../core/CapsLabel.svelte";
  import { CHOICES, TRIAGED } from "./audit.js";
  import { render } from "./markdown.js";

  /** @type {{finding: import("./audit.js").Finding, draft: {choice: string, note: string}, state?: string, message?: string, onChoose?: (choice: string) => void, onNote?: (text: string) => void, onCommit?: () => void}} */
  let { finding, draft, state = "", message = "", onChoose, onNote, onCommit } = $props();

  const STATES = { unsaved: "Not saved", saving: "Saving…", saved: "Saved", failed: "Not saved", reloaded: "Not saved" };
  const LABELS = { accept: "Accept", reject: "Reject", defer: "Defer" };

  // Line numbers here would count from the card, not the file.
  const prose = (text) => render(text).body.replace(/ data-line(?:-end)?="\d+"/g, "");

  let fields = $derived(finding.fields);
  let triaged = $derived(Boolean(draft.choice || draft.note.trim()));
</script>

<article class="card" class:triaged data-finding={finding.id} data-line={finding.from} data-line-end={finding.to}>
  <header class="head">
    <CapsLabel>{finding.id}</CapsLabel>
    <span class="confidence" data-confidence={fields.confidence}>{fields.confidence || "unrated"} confidence</span>
    <span class="state" data-state={state}>{STATES[state] ?? (TRIAGED[draft.choice] ?? (triaged ? "Note only" : "Untriaged"))}</span>
  </header>
  <h2 class="title">{finding.title}</h2>
  <dl class="fields">
    {#if fields.locations.length > 0}
      <dt>Locations</dt>
      <dd class="locations">
        {#each fields.locations as location}<code>{location}</code>{/each}
      </dd>
    {/if}
    {#if fields.evidence}
      <dt>Evidence</dt>
      <dd class="markdown">{@html prose(fields.evidence)}</dd>
    {/if}
    {#if fields.verdict}
      <dt>Verdict</dt>
      <dd>{fields.verdict}</dd>
    {/if}
    {#if fields.fix}
      <dt>Proposed fix</dt>
      <dd class="markdown">{@html prose(fields.fix)}</dd>
    {/if}
    {#if fields.risk}
      <dt>Behaviour risk</dt>
      <dd>{fields.risk}</dd>
    {/if}
    {#if fields.source}
      <dt>Source</dt>
      <dd>{fields.source}</dd>
    {/if}
  </dl>
  <div class="choices" role="group" aria-label="Triage for {finding.id}">
    {#each CHOICES as choice}
      <button
        type="button"
        class="choice"
        class:chosen={draft.choice === choice}
        aria-pressed={draft.choice === choice}
        data-choice={choice}
        onclick={() => onChoose?.(choice)}>{LABELS[choice]}</button>
    {/each}
  </div>
  <label class="note">
    <CapsLabel tone="dim">Note</CapsLabel>
    <textarea
      rows="2"
      value={draft.note}
      oninput={(event) => onNote?.(event.currentTarget.value)}
      onblur={() => onCommit?.()}
      onkeydown={(event) => {
        if (event.key === "Enter" && (event.metaKey || event.ctrlKey)) {
          event.preventDefault();
          onCommit?.();
        }
      }}></textarea>
  </label>
  {#if message}
    <p class="message" class:failed={state === "failed"} role="status">{message}</p>
  {/if}
</article>

<style>
  .card {
    display: flex;
    flex-direction: column;
    gap: calc(12px * var(--ui-scale));
    padding: calc(18px * var(--ui-scale)) calc(20px * var(--ui-scale)) calc(20px * var(--ui-scale));
    border: var(--border-width) solid var(--border-subtle);
    box-shadow: var(--shadow-offset) var(--border-subtle);
  }

  .card:focus-within {
    border-color: var(--accent-action);
    box-shadow: var(--shadow-focus-md);
  }

  .head {
    display: flex;
    align-items: center;
    gap: calc(12px * var(--ui-scale));
  }

  .confidence {
    font: var(--machine-sm);
    color: var(--text-muted);
  }

  .confidence[data-confidence="high"] {
    color: var(--artifact-audit);
  }

  .state {
    margin-left: auto;
    font: var(--machine-sm);
    color: var(--text-muted);
  }

  .triaged .state,
  .state[data-state="saved"] {
    color: var(--state-success);
  }

  .state[data-state="unsaved"],
  .state[data-state="reloaded"],
  .state[data-state="saving"] {
    color: var(--state-waiting);
  }

  .state[data-state="failed"],
  .message.failed {
    color: var(--state-failed);
  }

  .message {
    margin: 0;
    font: var(--machine-sm);
    color: var(--state-waiting);
  }

  .title {
    margin: 0;
    font: var(--display-sm);
    letter-spacing: var(--display-tracking);
    color: var(--text-primary);
    overflow-wrap: anywhere;
  }

  .fields {
    display: grid;
    grid-template-columns: max-content minmax(0, 1fr);
    gap: calc(6px * var(--ui-scale)) calc(14px * var(--ui-scale));
    margin: 0;
    font: var(--machine-md);
  }

  dt {
    font: var(--machine-sm);
    color: var(--text-muted);
  }

  dd {
    margin: 0;
    color: var(--text-secondary);
    overflow-wrap: anywhere;
  }

  dd :global(p) {
    margin: 0;
  }

  dd :global(p + p) {
    margin-top: calc(6px * var(--ui-scale));
  }

  .locations {
    display: flex;
    flex-wrap: wrap;
    gap: calc(6px * var(--ui-scale)) calc(10px * var(--ui-scale));
  }

  .locations code {
    font: var(--machine-sm);
    color: var(--accent-literal);
  }

  .choices {
    display: flex;
    flex-wrap: wrap;
    gap: calc(8px * var(--ui-scale));
  }

  .choice {
    min-width: calc(88px * var(--ui-scale));
    padding: calc(8px * var(--ui-scale)) calc(14px * var(--ui-scale));
    border: var(--border-width) solid var(--border-default);
    background: transparent;
    font: var(--machine-bold);
    color: var(--text-primary);
    cursor: pointer;
  }

  .choice:hover {
    background: var(--wash-selected);
    border-color: var(--accent-action);
  }

  .choice.chosen {
    border-color: var(--accent-action);
    background: color-mix(in srgb, var(--accent-action) 14%, transparent);
  }

  .note {
    display: flex;
    flex-direction: column;
    gap: calc(6px * var(--ui-scale));
  }

  textarea {
    width: 100%;
    box-sizing: border-box;
    min-height: calc(52px * var(--ui-scale));
    padding: calc(8px * var(--ui-scale)) calc(10px * var(--ui-scale));
    border: var(--border-width) solid var(--border-default);
    background: var(--surface-terminal);
    color: var(--text-primary);
    font: var(--machine-md);
    resize: vertical;
  }

  textarea:focus {
    outline: none;
    border-color: var(--accent-action);
  }

  :global(html[data-narrow]) .fields {
    grid-template-columns: minmax(0, 1fr);
  }
</style>
