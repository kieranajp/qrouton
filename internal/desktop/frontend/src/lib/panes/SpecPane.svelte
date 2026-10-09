<script>
  import { ARTIFACT_KINDS } from "../bridge/generated.js";
  import Button from "../core/Button.svelte";
  import CapsLabel from "../core/CapsLabel.svelte";
  import Chip from "../core/Chip.svelte";
  import { tick, untrack } from "svelte";
  import ArtifactPane from "./ArtifactPane.svelte";
  import QuestionCard from "./QuestionCard.svelte";
  import ReaderPips from "./ReaderPips.svelte";
  import { diagrams, links, viewport } from "./actions.js";
  import { clampedSpan, partition, screenFor } from "./deck.js";
  import { deck } from "./deck.svelte.js";
  import MarkdownPane from "./MarkdownPane.svelte";
  import { render } from "./markdown.js";
  import { reader, scrolls } from "./reader.svelte.js";
  import { parseSpec } from "./spec.js";
  import "./markdown.css";

  /** @type {{doc: {text: string, format: string, source: string, path?: string, kind?: string, line?: number, to?: number, viewportEpoch?: number}, id: string, active?: boolean, scrollRoot?: HTMLElement, onScroller?: (element: HTMLElement | null) => void}} */
  let { doc, id, active = false, scrollRoot, onScroller } = $props();

  let rendered = $derived(render(doc.text));
  let spec = $derived(parseSpec(doc.text));
  let heading = $derived(rendered.title || (doc.source ? doc.source.split("/").pop() : ""));

  let slides = $derived(slidesOf(spec));
  // Ranges no screen shows still have to claim their blocks, or they would land in the overview.
  let parts = $derived(partition(rendered.body, [...slides, ...unshown(spec)]));

  let drafts = $derived(spec.open.map((question) => draftOf(question)));
  let answered = $derived(drafts.filter((draft) => draft.letter || draft.note.trim()).length);
  let nextOpen = $derived(drafts.findIndex((draft) => !draft.letter && !draft.note.trim()));

  /** @param {ReturnType<typeof parseSpec>} parsed */
  function slidesOf(parsed) {
    const questions =
      parsed.open.length > 0 && parsed.openSection
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
    if (parsed.open.length === 0 && parsed.openSection) hidden.push(parsed.openSection);
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

  /** @param {number} index @param {string} letter */
  function pick(index, letter) {
    void index;
    void letter;
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
    epoch: () => doc.viewportEpoch,
    marking: () => !untrack(() => at.retired),
  });
</script>

<svelte:window onkeydown={onKey} />

{#if !spec.isSpec}
  <MarkdownPane {doc} {id} {active} {scrollRoot} />
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
          <MarkdownPane {doc} {id} {active} {scrollRoot} bare onMeasure={spy} />
        </div>
      {:else}
        <div
          class="deck"
          bind:this={sheet}
          data-document-source={doc.source}
          use:links={doc.source}
          use:diagrams={{ id, text: doc.text }}
          use:port={{ id, active, scrollRoot, key: at.current, request: doc.viewportEpoch }}>
          <section class="screen hero" data-screen="overview" hidden={viewing !== 0}>
            <CapsLabel>Spec · {spec.decisions.length} {spec.decisions.length === 1 ? "decision" : "decisions"}</CapsLabel>
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
                      <span class="name">{decision.label}</span>
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
                  {#each spec.open as question, at (question.id + question.heading)}
                    <QuestionCard
                      {question}
                      draft={drafts[at]}
                      focused={focus === at}
                      onFocus={() => (focus = at)}
                      onPick={(letter) => pick(at, letter)} />
                  {/each}
                </div>
              {:else}
                {#if slide.kind !== "section"}
                  <div class="crumb">
                    <CapsLabel>{slide.kind === "question" ? "Resolved question" : "Decision"}</CapsLabel>
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
    padding: 0 var(--pane-pad) 26px;
  }

  .reading .display-lg {
    margin-top: 4px;
  }

  .hero {
    padding-top: 18px;
  }

  .hero .display-lg {
    margin: 14px 0 20px;
    max-width: 26ch;
  }

  .hero .lead :global(p) {
    font: var(--machine-lg);
    font-size: 15px;
    line-height: 1.7;
    color: var(--text-secondary);
    max-width: 68ch;
  }

  .tally {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 14px;
    margin-top: 26px;
    padding-left: var(--gutter);
  }

  .tally .says {
    font: var(--machine-bold);
    font-size: 14px;
    color: var(--text-primary);
  }

  .screen > :global(.caps),
  .crumb {
    padding-left: var(--gutter);
  }

  .crumb {
    display: flex;
    align-items: center;
    gap: 12px;
    padding-top: 18px;
  }

  .count {
    font: var(--machine-sm);
    color: var(--text-muted);
  }

  .display-lg,
  .display-md {
    margin: 12px 0 18px;
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
    gap: 22px;
    padding-left: var(--gutter);
  }

  .rows {
    list-style: none;
    margin: 26px 0 0;
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
    gap: 14px;
    width: 100%;
    padding: 11px 14px;
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

  .row .name {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    color: var(--text-primary);
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

  @media (max-width: 420px) {
    .hero {
      padding-top: 6px;
    }

    .display-lg {
      font: var(--display-md);
    }

    .display-md {
      font: var(--display-sm);
    }

    .cards,
    .tally {
      padding-left: 0;
    }
  }
</style>
