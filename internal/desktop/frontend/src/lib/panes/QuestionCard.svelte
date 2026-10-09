<script>
  import CapsLabel from "../core/CapsLabel.svelte";
  import { render } from "./markdown.js";

  /** @type {{question: import("./spec.js").Question, draft: {letter: string, note: string}, focused?: boolean, readonly?: boolean, state?: string, message?: string, onPick?: (letter: string) => void, onNote?: (text: string) => void, onCommit?: () => void, onFocus?: () => void}} */
  let { question, draft, focused = false, readonly = false, state = "", message = "", onPick, onNote, onCommit, onFocus } = $props();

  const STATES = { unsaved: "Not saved", saving: "Saving…", saved: "Saved", failed: "Not saved", reloaded: "Not saved" };

  // Line numbers here would count from the card, not the file.
  const unnumbered = (text) => render(text).body.replace(/ data-line(?:-end)?="\d+"/g, "");
  // A fence keeps its line in the file so the diagram renderer can find it.
  const anchored = (text, first) =>
    render(text).body.replace(/<(\w+)([^>]*)>/g, (tag, name, attrs) =>
      attrs.includes("data-line")
        ? `<${name}${attrs.replace(/ data-line(-end)?="(\d+)"/g, (_, end, at) =>
            name === "pre" ? ` data-line${end ?? ""}="${Number(at) + first - 1}"` : "")}>`
        : tag);
  const inline = (text) => unnumbered(text).replace(/^<p>|<\/p>\s*$/g, "");

  let context = $derived(question.context ? anchored(question.context, question.contextFrom) : "");
  let answered = $derived(Boolean(draft.letter || draft.note.trim()));
</script>

<article
  class="card"
  class:focused
  class:answered
  data-question={question.id}
  tabindex="-1"
  onfocusin={() => onFocus?.()}>
  <header class="head">
    <CapsLabel>{question.id}</CapsLabel>
    <span class="state" data-state={state}>{STATES[state] ?? (answered ? "Answered" : "Open")}</span>
  </header>
  <h2 class="question">{question.heading}</h2>
  {#if context}
    <div class="markdown context">{@html context}</div>
  {/if}
  <div class="options" role="group" aria-label="Options for {question.id}">
    {#each question.options as option}
      <button
        type="button"
        class="option"
        class:chosen={draft.letter === option.letter}
        aria-pressed={draft.letter === option.letter}
        disabled={readonly}
        data-letter={option.letter}
        onclick={() => onPick?.(option.letter)}>
        <span class="letter">{option.letter}</span>
        <span class="text">
          <span class="label">{@html inline(option.text)}</span>
          {#if option.recommended}
            <span class="recommended">
              <span class="badge">Recommended</span>
              {#if option.reason}<span class="reason">{@html inline(option.reason)}</span>{/if}
            </span>
          {/if}
        </span>
      </button>
    {/each}
  </div>
  <label class="note">
    <CapsLabel tone="dim">{question.options.length > 0 ? "Note, or an answer of your own" : "Answer"}</CapsLabel>
    <textarea
      rows="2"
      value={draft.note}
      disabled={readonly}
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
    outline: none;
  }

  .card.focused {
    border-color: var(--accent-action);
    box-shadow: var(--shadow-focus-md);
  }

  .head {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: calc(12px * var(--ui-scale));
  }

  .state {
    font: var(--machine-sm);
    color: var(--text-muted);
  }

  .answered .state,
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

  .question {
    margin: 0;
    font: var(--display-sm);
    letter-spacing: var(--display-tracking);
    color: var(--text-primary);
    overflow-wrap: anywhere;
  }

  .context :global(p) {
    margin: 0;
    font: var(--machine-md);
    color: var(--text-secondary);
  }

  .options {
    display: flex;
    flex-direction: column;
    gap: calc(8px * var(--ui-scale));
  }

  .option {
    display: flex;
    align-items: flex-start;
    gap: calc(14px * var(--ui-scale));
    width: 100%;
    min-height: calc(48px * var(--ui-scale));
    padding: calc(12px * var(--ui-scale)) calc(14px * var(--ui-scale));
    border: var(--border-width) solid var(--border-default);
    background: transparent;
    font: var(--machine-md);
    color: var(--text-primary);
    text-align: left;
    cursor: pointer;
  }

  .option:not(:disabled):hover {
    background: var(--wash-selected);
    border-color: var(--accent-action);
  }

  .option:disabled {
    cursor: default;
  }

  .option.chosen {
    border-color: var(--accent-action);
    background: color-mix(in srgb, var(--accent-action) 14%, transparent);
  }

  .letter {
    flex: none;
    min-width: 2ch;
    font: var(--machine-bold);
    font-size: calc(15px * var(--ui-scale));
    color: var(--accent-action);
  }

  .chosen .letter {
    color: var(--text-primary);
  }

  .text {
    display: flex;
    flex-direction: column;
    gap: calc(6px * var(--ui-scale));
    min-width: 0;
    overflow-wrap: anywhere;
  }

  .recommended {
    display: flex;
    flex-wrap: wrap;
    align-items: baseline;
    gap: calc(8px * var(--ui-scale));
    font: var(--machine-sm);
    color: var(--text-secondary);
  }

  .badge {
    padding: 2px calc(6px * var(--ui-scale));
    background: var(--accent-label);
    color: var(--text-on-accent);
    font: var(--instruction);
    letter-spacing: var(--instruction-tracking);
    text-transform: uppercase;
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
</style>
