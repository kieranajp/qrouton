<script>
  import { ARTIFACT_KINDS } from "../bridge/generated.js";
  import Button from "../core/Button.svelte";
  import CapsLabel from "../core/CapsLabel.svelte";
  import Chip from "../core/Chip.svelte";
  import { tick, untrack } from "svelte";
  import { SvelteMap } from "svelte/reactivity";
  import { copyText } from "../wails.js";
  import ArtifactPane from "./ArtifactPane.svelte";
  import QuestionCard from "./QuestionCard.svelte";
  import ReaderPips from "./ReaderPips.svelte";
  import { diagrams, links, viewport } from "./actions.js";
  import { clampedSpan, partition, screenFor } from "./deck.js";
  import { deck } from "./deck.svelte.js";
  import MarkdownPane from "./MarkdownPane.svelte";
  import { render } from "./markdown.js";
  import { reader, scrolls } from "./reader.svelte.js";
  import { freshContent, saveSpec, sendSpecAnswers } from "./spec-calls.js";
  import { draftKey, parseSpec, reapply, spliceAnswer } from "./spec.js";
  import "./markdown.css";

  /** @type {{doc: {text: string, hash?: string, format: string, source: string, path?: string, kind?: string, line?: number, to?: number, viewportEpoch?: number}, id: string, active?: boolean, scrollRoot?: HTMLElement, onScroller?: (element: HTMLElement | null) => void}} */
  let { doc, id, active = false, scrollRoot, onScroller } = $props();

  // The text the pane splices into: the last push, or the pane's own save, or
  // what it fetched after a refused one, whichever came last.
  let source = $state(untrack(() => ({ text: doc.text, hash: doc.hash ?? "" })));
  let epoch = $state(untrack(() => doc.viewportEpoch ?? 0));
  let pushed = untrack(() => doc.hash ?? doc.text);
  $effect(() => {
    const text = doc.text;
    const hash = doc.hash ?? "";
    const at = doc.viewportEpoch ?? 0;
    untrack(() => {
      if (at > epoch) epoch = at;
      if ((hash || text) === pushed) return;
      pushed = hash || text;
      source = { text, hash };
    });
  });
  let shown = $derived({ ...doc, text: source.text, viewportEpoch: epoch });

  /** @type {SvelteMap<string, {id: string, heading: string, letter: string, note: string}>} */
  const pending = new SvelteMap();
  /** @type {SvelteMap<string, {state: string, message?: string}>} */
  const saves = new SvelteMap();
  let queue = Promise.resolve();

  let rendered = $derived(render(source.text));
  let spec = $derived(parseSpec(source.text));
  let laid = $derived(reapply(spec, pending));
  let heading = $derived(rendered.title || (doc.source ? doc.source.split("/").pop() : ""));

  let slides = $derived(slidesOf(spec));
  // Ranges no screen shows still have to claim their blocks, or they would land in the overview.
  let parts = $derived(partition(rendered.body, [...slides, ...unshown(spec)]));

  let drafts = $derived(laid.questions.map(({ question, draft }) => draft ?? draftOf(question)));
  let answered = $derived(drafts.filter((draft) => draft.letter || draft.note.trim()).length);
  let nextOpen = $derived(drafts.findIndex((draft) => !draft.letter && !draft.note.trim()));

  /** @param {ReturnType<typeof parseSpec>} parsed */
  function slidesOf(parsed) {
    const questions =
      (parsed.open.length > 0 || laid.detached.length > 0) && parsed.openSection
        ? [{ kind: "questions", name: "Open questions", ...parsed.openSection }]
        : [];
    const decisions = parsed.decisions.map((decision) => ({
      kind: decision.kind,
      name: decision.id ? `${decision.id} — ${decision.label}` : decision.label,
      from: decision.from,
      to: decision.to,
    }));
    const others = parsed.sections.map((section) => ({ kind: "section", ...section }));
    return [...questions, ...decisions, ...others];
  }

  /** @param {ReturnType<typeof parseSpec>} parsed */
  function unshown(parsed) {
    const hidden = [];
    if (parsed.open.length === 0 && laid.detached.length === 0 && parsed.openSection)
      hidden.push(parsed.openSection);
    if (parsed.decisionsSection) {
      const first = parsed.decisions[0];
      hidden.push({
        from: parsed.decisionsSection.from,
        to: first ? first.from - 1 : parsed.decisionsSection.to,
      });
    }
    return hidden;
  }

  /** @param {import("./spec.js").Question} question */
  function draftOf(question) {
    return { letter: question.answer?.letter ?? "", note: question.answer?.note ?? "" };
  }

  /** @type {HTMLElement | undefined} */
  let sheet = $state();
  /** @type {HTMLElement | undefined} */
  let reading = $state();
  let focus = $state(0);

  const at = deck({
    slides: () => slides,
    line: () => doc.line ?? 0,
    followed: () => 0,
    live: () => false,
    body: () => sheet,
  });

  const view = reader({
    structured: "spec",
    doc: () => doc,
    reload: () => at.reload(),
  });

  $effect(() => {
    at.clamp(slides.length);
  });

  $effect(() => {
    if (focus >= spec.open.length) focus = Math.max(0, spec.open.length - 1);
  });

  scrolls({
    reading: () => view.reading,
    structured: () => sheet,
    document: () => reading,
    when: () => slides.length > 0,
    onScroller: () => onScroller,
  });

  let scrolled = $state(0);
  let viewing = $derived(view.reading ? scrolled : at.current);
  let onQuestions = $derived(!view.reading && slides[at.current - 1]?.kind === "questions");
  let pips = $derived([
    { label: "Overview", summary: true },
    ...slides.map((slide) => ({
      label: slide.name,
      summary: slide.kind === "section",
      color:
        slide.kind === "questions"
          ? answered === spec.open.length
            ? "var(--state-success)"
            : "var(--state-waiting)"
          : slide.kind === "section"
            ? undefined
            : "var(--artifact-spec)",
    })),
  ]);
  let counter = $derived(viewing === 0 ? "Overview" : (slides[viewing - 1]?.name ?? ""));

  /** @param {{intervals: {line: number, to: number}[]}} state */
  function spy(state) {
    if (state.intervals.length === 0) return;
    const ended =
      scrollRoot && scrollRoot.scrollTop + scrollRoot.clientHeight >= scrollRoot.scrollHeight - 2;
    const line = ended ? state.intervals.at(-1).to : state.intervals[0].line;
    scrolled = screenFor(slides, line);
  }

  function reach(screen) {
    if (!view.reading) {
      at.show(screen);
      return;
    }
    const from = screen === 0 ? spec.preamble.from : slides[screen - 1].from;
    const blocks = [
      .../** @type {NodeListOf<HTMLElement>} */ (reading?.querySelectorAll("[data-line]") ?? []),
    ];
    const target = blocks.find((block) => Number(block.dataset.line) >= from) ?? blocks[0];
    target?.scrollIntoView({ block: "start" });
  }

  /** @param {number} index */
  async function focusCard(index) {
    focus = Math.max(0, Math.min(index, spec.open.length - 1));
    await tick();
    sheet
      ?.querySelector(`[data-question="${spec.open[focus]?.id}"]`)
      ?.scrollIntoView({ block: "nearest" });
  }

  function showQuestions(index) {
    const screen = slides.findIndex((slide) => slide.kind === "questions") + 1;
    if (screen > 0) at.show(screen);
    focusCard(index);
  }

  /** @param {number} index @param {Partial<{letter: string, note: string}>} change */
  function edit(index, change) {
    const question = spec.open[index];
    if (!question) return "";
    const key = draftKey(question);
    sent = null;
    pending.set(key, { id: question.id, heading: question.heading, ...drafts[index], ...change });
    return key;
  }

  /** @param {number} index @param {string} letter */
  function pick(index, letter) {
    commit(edit(index, { letter: drafts[index]?.letter === letter ? "" : letter }));
  }

  /** @param {number} index @param {string} note */
  function note(index, note) {
    saves.set(edit(index, { note }), { state: "unsaved" });
  }

  /** Saves run one at a time, so each splices into the text the last one left. */
  function commit(key) {
    if (!key || !pending.has(key)) return;
    queue = queue.then(() => save(key));
  }

  async function save(key) {
    const draft = pending.get(key);
    const question = spec.open.find((open) => draftKey(open) === key);
    if (!draft || !question) return;
    const text = spliceAnswer(source.text, question, draft);
    if (text === source.text) {
      pending.delete(key);
      saves.delete(key);
      return;
    }
    const hash = source.hash;
    saves.set(key, { state: "saving" });
    const saved = await saveSpec(id, hash, text);
    if (saved.ok) {
      source = { text, hash: saved.value };
      if (pending.get(key) === draft) pending.delete(key);
      saves.set(key, { state: pending.has(key) ? "unsaved" : "saved" });
      return;
    }
    // Go refuses a stale hash; a fresh read tells that apart from a real failure.
    const fresh = await freshContent(id);
    if (fresh.ok && fresh.value.hash !== hash) {
      source = { text: fresh.value.text, hash: fresh.value.hash };
      epoch = Math.max(epoch, fresh.value.viewportEpoch ?? 0);
      saves.set(key, {
        state: "reloaded",
        message: "The spec changed on disk, so it reloaded. Your answer is kept here: pick again or press ⌘↵ to save it.",
      });
      return;
    }
    saves.set(key, { state: "failed", message: `Could not save: ${saved.error?.message ?? saved.error}` });
  }

  let blocked = $derived.by(() => {
    const states = [...saves.values()].map((save) => save.state);
    if (states.includes("saving")) return "Wait for the save to finish.";
    if (states.includes("failed") || states.includes("reloaded"))
      return "An answer did not save. Save it again first.";
    if (laid.detached.length > 0) return "Copy or discard the detached answers first.";
    if (pending.size > 0) return "Save your typed answer first: click outside the field or press ⌘↵.";
    if (answered === 0) return "Answer at least one question first.";
    return "";
  });
  let sent = $state(/** @type {{ok: boolean, message: string} | null} */ (null));

  async function send() {
    if (blocked) return;
    const typed = await sendSpecAnswers(id);
    sent = typed.ok
      ? { ok: true, message: "Typed into the conversation." }
      : { ok: false, message: `Could not type into the conversation: ${typed.error?.message ?? typed.error}` };
  }

  /** @param {{id: string, heading: string}} draft */
  function discard(draft) {
    pending.delete(draftKey(draft));
    saves.delete(draftKey(draft));
  }

  /** @param {KeyboardEvent} event */
  function onKey(event) {
    if (!active || slides.length === 0) return;
    if (event.metaKey || event.ctrlKey || event.altKey) return;
    const from = /** @type {HTMLElement} */ (event.target);
    const field = /^(input|textarea|select)$/i.test(from?.tagName ?? "");
    if (from?.isContentEditable || field) return;
    const key = event.key;
    if (key === "ArrowRight") at.show(at.current + 1);
    else if (key === "ArrowLeft") at.show(at.current - 1);
    else if (onQuestions && (key === "ArrowDown" || key === "j")) focusCard(focus + 1);
    else if (onQuestions && (key === "ArrowUp" || key === "k")) focusCard(focus - 1);
    else if (onQuestions && /^[a-z]$/i.test(key) && spec.open[focus]?.options.some((option) => option.letter === key.toUpperCase()))
      pick(focus, key.toUpperCase());
    else return;
    event.preventDefault();
  }

  const port = viewport({
    span: () => clampedSpan(doc, at.current > 0 ? slides[at.current - 1] : spec.preamble),
    epoch: () => epoch,
    marking: () => !untrack(() => at.retired),
  });
</script>

<svelte:window onkeydown={onKey} />

{#if !spec.isSpec}
  <MarkdownPane doc={shown} {id} {active} {scrollRoot} />
{:else}
  <ArtifactPane
    {doc}
    pips
    structured="spec"
    label="Spec"
    mode={view.mode}
    onMode={(next) => (view.mode = next)}>
    {#snippet tag()}
      <Chip>{doc.kind ?? ARTIFACT_KINDS.SPEC}</Chip>
    {/snippet}
    {#snippet body()}
      {#if view.reading}
        <div class="reading" bind:this={reading}>
          <h1 class="display-lg">{spec.title || heading}</h1>
          <MarkdownPane doc={shown} {id} {active} {scrollRoot} bare onMeasure={spy} />
        </div>
      {:else}
        <div
          class="deck"
          bind:this={sheet}
          data-document-source={doc.source}
          use:links={doc.source}
          use:diagrams={{ id, text: source.text }}
          use:port={{ id, active, scrollRoot, key: at.current, request: epoch }}>
          <section class="screen hero" data-screen="overview" hidden={viewing !== 0}>
            <CapsLabel>Spec · {spec.decisionCount} {spec.decisionCount === 1 ? "decision" : "decisions"}</CapsLabel>
            <h1 class="display-lg">{spec.title || heading}</h1>
            <div class="markdown lead">{@html parts.preamble}</div>
            <div class="tally" data-tally>
              {#if spec.open.length === 0}
                <span class="says">No open questions</span>
              {:else}
                <span class="says">{answered} of {spec.open.length} answered</span>
                {#if nextOpen >= 0}
                  <Button variant="outline" size="sm" onclick={() => showQuestions(nextOpen)}>
                    Next open question →
                  </Button>
                {:else}
                  <Button variant="outline" size="sm" onclick={() => showQuestions(0)}>Review answers →</Button>
                {/if}
              {/if}
            </div>
            {#if spec.decisions.length > 0}
              <ol class="rows">
                {#each spec.decisions as decision, index}
                  <li>
                    <button
                      type="button"
                      class="row"
                      onclick={() => at.show(slides.findIndex((slide) => slide.from === decision.from) + 1)}>
                      <span class="index">{decision.id || index + 1}</span>
                      <span class="text">
                        <span class="name">{decision.label}{decision.kind === "group" && decision.count > 0 ? ` · ${decision.count} ${decision.count === 1 ? "decision" : "decisions"}` : ""}</span>
                        {#each decision.leads as lead}
                          <span class="lead-line">{lead}</span>
                        {/each}
                      </span>
                    </button>
                  </li>
                {/each}
              </ol>
            {/if}
          </section>
          {#each slides as slide, index}
            <section class="screen" data-screen={slide.kind === "questions" ? "questions" : slide.name} hidden={viewing !== index + 1}>
              {#if slide.kind === "questions"}
                <div class="crumb">
                  <CapsLabel>Design questions</CapsLabel>
                  <span class="count">{answered} of {spec.open.length} answered</span>
                </div>
                <h1 class="display-md">Open questions</h1>
                <div class="cards">
                  {#each spec.open as question, at (draftKey(question))}
                    <QuestionCard
                      {question}
                      draft={drafts[at]}
                      focused={focus === at}
                      state={saves.get(draftKey(question))?.state}
                      message={saves.get(draftKey(question))?.message}
                      onFocus={() => (focus = at)}
                      onPick={(letter) => pick(at, letter)}
                      onNote={(text) => note(at, text)}
                      onCommit={() => commit(draftKey(question))} />
                  {/each}
                </div>
                {#if laid.detached.length > 0}
                  <div class="detached" data-detached>
                    <CapsLabel>Detached answers</CapsLabel>
                    <p class="hint">These questions were renamed or removed while you were answering. Your text is kept here.</p>
                    {#each laid.detached as draft (draftKey(draft))}
                      <div class="orphan" data-orphan={draft.id}>
                        <span class="was">{draft.id} — {draft.heading}</span>
                        <p class="kept">{[draft.letter, draft.note].filter(Boolean).join(" — ")}</p>
                        <div class="actions">
                          <Button variant="outline" size="sm" onclick={() => copyText([draft.letter, draft.note].filter(Boolean).join("\n\n"))}>Copy</Button>
                          <Button variant="ghost" size="sm" onclick={() => discard(draft)}>Discard</Button>
                        </div>
                      </div>
                    {/each}
                  </div>
                {/if}
              {:else}
                {#if slide.kind !== "section"}
                  <div class="crumb">
                    <CapsLabel>{slide.kind === "question" ? "Resolved question" : "Decisions"}</CapsLabel>
                  </div>
                {/if}
                <h1 class="display-md">{slide.name}</h1>
                <div class="markdown" class:lifted={slide.kind !== "decision"}>{@html parts.sections[index].opening}</div>
                <div class="markdown">{@html parts.sections[index].body}</div>
              {/if}
            </section>
          {/each}
        </div>
      {/if}
    {/snippet}
    {#snippet bar()}
      {#if spec.open.length > 0}
        <div class="send">
          <span class="send-button" title={blocked || null}>
            <Button
              variant={answered === spec.open.length && !blocked ? "primary" : "outline"}
              size="sm"
              disabled={Boolean(blocked)}
              data-send
              onclick={send}>Type 'answers are in' into the conversation</Button>
          </span>
          <span class="why" class:failed={sent && !sent.ok} role="status">{blocked || sent?.message || ""}</span>
        </div>
      {/if}
    {/snippet}
    {#snippet controls()}
      <ReaderPips entries={pips} current={viewing} onSelect={reach} />
    {/snippet}
    {#snippet counter()}
      <span class="counter" title={counter}>{counter}</span>
      {#if !view.reading}
        <div class="steps">
          <Button
            variant="ghost"
            size="sm"
            aria-label="Previous screen"
            disabled={at.current === 0}
            onclick={() => at.show(at.current - 1)}>←</Button>
          <Button
            variant="ghost"
            size="sm"
            aria-label="Next screen"
            disabled={at.current === slides.length}
            onclick={() => at.show(at.current + 1)}>→</Button>
        </div>
      {/if}
    {/snippet}
  </ArtifactPane>
{/if}

<style>
  .deck,
  .reading {
    flex: 1;
    min-height: 0;
    overflow-y: auto;
    padding: 0 var(--pane-pad) calc(26px * var(--ui-scale));
  }

  .reading .display-lg {
    margin-top: calc(4px * var(--ui-scale));
  }

  .hero {
    padding-top: calc(18px * var(--ui-scale));
  }

  .hero .display-lg {
    margin: calc(14px * var(--ui-scale)) 0 calc(20px * var(--ui-scale));
    max-width: 26ch;
  }

  .hero .lead :global(p) {
    font: var(--machine-lg);
    font-size: calc(15px * var(--ui-scale));
    line-height: 1.7;
    color: var(--text-secondary);
    max-width: 68ch;
  }

  .tally {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: calc(14px * var(--ui-scale));
    margin-top: calc(26px * var(--ui-scale));
    padding-left: var(--gutter);
  }

  .tally .says {
    font: var(--machine-bold);
    font-size: calc(14px * var(--ui-scale));
    color: var(--text-primary);
  }

  .screen > :global(.caps),
  .crumb {
    padding-left: var(--gutter);
  }

  .crumb {
    display: flex;
    align-items: center;
    gap: calc(12px * var(--ui-scale));
    padding-top: calc(18px * var(--ui-scale));
  }

  .count {
    font: var(--machine-sm);
    color: var(--text-muted);
  }

  .display-lg,
  .display-md {
    margin: calc(12px * var(--ui-scale)) 0 calc(18px * var(--ui-scale));
    padding-left: var(--gutter);
    letter-spacing: var(--display-tracking);
    color: var(--text-primary);
    overflow-wrap: anywhere;
  }

  .display-lg {
    font: var(--display-lg);
  }

  .display-md {
    font: var(--display-md);
  }

  .cards {
    display: flex;
    flex-direction: column;
    gap: calc(22px * var(--ui-scale));
    padding-left: var(--gutter);
  }

  .rows {
    list-style: none;
    margin: calc(26px * var(--ui-scale)) 0 0;
    padding: 0;
    border: var(--border-width) solid var(--border-subtle);
    box-shadow: var(--shadow-offset) var(--border-subtle);
  }

  .rows li + li .row {
    border-top: var(--border-width) solid var(--border-subtle);
  }

  .row {
    display: flex;
    align-items: center;
    gap: calc(14px * var(--ui-scale));
    width: 100%;
    padding: calc(11px * var(--ui-scale)) calc(14px * var(--ui-scale));
    border: 0;
    background: transparent;
    font: var(--machine-md);
    color: var(--text-secondary);
    text-align: left;
    cursor: pointer;
  }

  .row:hover {
    background: var(--wash-selected);
  }

  .row .index {
    min-width: 3ch;
    font: var(--machine-bold);
    color: var(--accent-action);
  }

  .row .text {
    display: flex;
    flex: 1;
    flex-direction: column;
    gap: calc(4px * var(--ui-scale));
    min-width: 0;
  }

  .row .name,
  .row .lead-line {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .row .name {
    color: var(--text-primary);
  }

  .row .lead-line {
    font: var(--machine-sm);
    color: var(--text-muted);
  }

  /* The pane names the section already; the heading stays measurable for the viewport. */
  .lifted {
    position: absolute;
    width: 1px;
    height: 1px;
    margin: 0;
    padding: 0;
    overflow: hidden;
    white-space: nowrap;
  }

  .send {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: calc(12px * var(--ui-scale));
    padding: calc(8px * var(--ui-scale)) var(--pane-pad);
    border-bottom: var(--border-width) solid var(--border-subtle);
  }

  .why {
    min-width: 0;
    font: var(--machine-sm);
    color: var(--text-muted);
  }

  .why.failed {
    color: var(--state-failed);
  }

  .detached {
    display: flex;
    flex-direction: column;
    gap: calc(12px * var(--ui-scale));
    margin-top: calc(26px * var(--ui-scale));
    padding-left: var(--gutter);
  }

  .hint,
  .kept {
    margin: 0;
    font: var(--machine-sm);
    color: var(--text-secondary);
    white-space: pre-wrap;
    overflow-wrap: anywhere;
  }

  .orphan {
    display: flex;
    flex-direction: column;
    gap: calc(8px * var(--ui-scale));
    padding: calc(12px * var(--ui-scale)) calc(14px * var(--ui-scale));
    border: var(--border-width) dashed var(--state-waiting);
  }

  .was {
    font: var(--machine-bold);
    color: var(--text-primary);
  }

  .actions {
    display: flex;
    gap: calc(8px * var(--ui-scale));
  }

  :global(html[data-narrow]) .detached {
    padding-left: 0;
  }

  :global(html[data-narrow]) .hero {
    padding-top: calc(6px * var(--ui-scale));
  }

  :global(html[data-narrow]) .display-lg {
    font: var(--display-md);
  }

  :global(html[data-narrow]) .display-md {
    font: var(--display-sm);
  }

  :global(html[data-narrow]) .cards,
  :global(html[data-narrow]) .tally {
    padding-left: 0;
  }
</style>
