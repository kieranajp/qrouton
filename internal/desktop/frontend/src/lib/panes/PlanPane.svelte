<script>
  import Button from "../core/Button.svelte";
  import CapsLabel from "../core/CapsLabel.svelte";
  import Chip from "../core/Chip.svelte";
  import { untrack } from "svelte";
  import ArtifactPane from "./ArtifactPane.svelte";
  import { diagrams, links, viewport } from "./actions.js";
  import { clampedSpan, counterFor, holding, partition, screenFor } from "./deck.js";
  import { deck } from "./deck.svelte.js";
  import MarkdownPane from "./MarkdownPane.svelte";
  import { render } from "./markdown.js";
  import { criteriaSpans, parsePlan } from "./plan.js";
  import { reader, scrolls } from "./reader.svelte.js";
  import "./markdown.css";

  const DOT = {
    met: "var(--state-success)",
    working: "var(--state-running)",
    "not-started": "var(--text-faint)",
  };
  const WORD = { met: "Met", working: "Working", "not-started": "Not started" };

  /** @type {{doc: {text: string, format: string, source: string, path?: string, kind?: string, line?: number, to?: number, viewportEpoch?: number}, id: string, active?: boolean, scrollRoot?: HTMLElement, agentWorking?: boolean, onScroller?: (element: HTMLElement | null) => void}} */
  let { doc, id, active = false, scrollRoot, agentWorking = false, onScroller } = $props();

  let rendered = $derived(render(doc.text));
  let plan = $derived(parsePlan(doc.text));
  let parts = $derived(partition(rendered.body, plan.slides, { criteria: criteriaSpans }));
  let heading = $derived(rendered.title || (doc.source ? doc.source.split("/").pop() : ""));

  let allMet = $derived(plan.phases.length > 0 && plan.phases.every((phase) => phase.state === "met"));
  // The phase the meter rests on, and with nothing unmet left to point at, the
  // last one. The bar names it from the phase itself: a screen counts sections
  // too, so a screen number is not a phase number and cannot index the phases.
  let metered = $derived(plan.phases.find((phase) => phase.state !== "met") ?? plan.phases.at(-1));
  // An agent is working somewhere in this session. Nothing here knows whether
  // it is working on this plan, and the bar must not say that it does. A deck
  // of nothing but sections has no meter, so it has nothing to report.
  let live = $derived((agentWorking || allMet) && Boolean(metered));

  /** @type {HTMLElement | undefined} */
  let sheet = $state();
  /** @type {HTMLElement | undefined} */
  let reading = $state();

  const at = deck({
    slides: () => plan.slides,
    line: () => doc.line ?? 0,
    followed: () => metered?.screen ?? 0,
    live: () => live,
    body: () => sheet,
  });

  const view = reader({
    structured: "plan",
    doc: () => doc,
    reload: () => at.reload(),
  });

  $effect(() => {
    const count = plan.slides.length;
    at.clamp(count);
  });

  scrolls({
    reading: () => view.reading,
    structured: () => sheet,
    document: () => reading,
    when: () => plan.slides.length > 0,
    onScroller: () => onScroller,
  });

  // The slide on screen, and the one the footer names: one value, so they
  // cannot drift apart. In Document mode nothing is hidden, so it is the slide
  // the reader has scrolled into rather than the one they selected.
  let scrolled = $state(0);
  let viewing = $derived(view.reading ? scrolled : at.current);

  /** @param {{intervals: {line: number, to: number}[]}} state */
  function spy(state) {
    if (state.intervals.length === 0) return;
    // The section under the top edge is the one being read — except at the very
    // bottom, which no later section can ever scroll up past.
    const ended =
      scrollRoot && scrollRoot.scrollTop + scrollRoot.clientHeight >= scrollRoot.scrollHeight - 2;
    const line = ended ? state.intervals.at(-1).to : state.intervals[0].line;
    scrolled = screenFor(plan.slides, line);
  }

  // One strip, two jobs: it selects a screen in the deck and scrolls to a
  // section in the document.
  function reach(screen) {
    if (!view.reading) {
      at.show(screen);
      return;
    }
    const from = screen === 0 ? plan.preamble.from : plan.slides[screen - 1].from;
    const blocks = [
      .../** @type {NodeListOf<HTMLElement>} */ (reading?.querySelectorAll("[data-line]") ?? []),
    ];
    const target = blocks.find((block) => Number(block.dataset.line) >= from) ?? blocks[0];
    target?.scrollIntoView({ block: "start" });
  }

  /** @param {KeyboardEvent} event */
  function onKey(event) {
    if (!active || plan.slides.length === 0) return;
    if (event.metaKey || event.ctrlKey || event.altKey) return;
    const from = /** @type {HTMLElement} */ (event.target);
    const field = /^(input|textarea|select)$/i.test(from?.tagName ?? "");
    const tick = /** A checkbox has no caret, so the deck keeps its arrow keys.
     * @type {HTMLInputElement} */ (from)?.type === "checkbox";
    if (from?.isContentEditable || (field && !tick)) return;
    if (event.key === "ArrowRight") at.show(at.current + 1);
    else if (event.key === "ArrowLeft") at.show(at.current - 1);
    else return;
    event.preventDefault();
  }

  const port = viewport({
    span: () =>
      clampedSpan(doc, at.current > 0 ? plan.slides[at.current - 1] : plan.preamble),
    epoch: () => doc.viewportEpoch,
    marking: () => !untrack(() => at.retired),
  });
</script>

<svelte:window onkeydown={onKey} />

{#if plan.slides.length === 0}
  <MarkdownPane {doc} {id} {active} {scrollRoot} />
{:else}
  <ArtifactPane
    {doc}
    structured="plan"
    label="Plan"
    mode={view.mode}
    onMode={(next) => (view.mode = next)}>
    {#snippet tag()}
      <Chip>{doc.kind ?? "PLAN"}</Chip>
    {/snippet}
    {#snippet body()}
      {#if view.reading}
        <div class="reading" bind:this={reading}>
          <!-- The renderer lifts the opening heading out of the body, so the
               document view states the plan's name itself. -->
          <h1 class="display-lg">{plan.title || heading}</h1>
          <MarkdownPane {doc} {id} {active} {scrollRoot} bare onMeasure={spy} />
        </div>
      {:else}
        <div
          class="deck"
          bind:this={sheet}
          data-document-source={doc.source}
          use:links={doc.source}
          use:diagrams={{ id, text: doc.text }}
          use:port={{ id, active, scrollRoot, key: at.current }}>
          <section class="screen hero" data-screen="overview" hidden={viewing !== 0}>
            <CapsLabel
              >Plan · {plan.phases.length}
              {plan.phases.length === 1 ? "phase" : "phases"}</CapsLabel>
            <h1 class="display-lg">{plan.title || heading}</h1>
            <div class="markdown lead">{@html parts.preamble}</div>
            <ol class="rows">
              {#each plan.phases as phase}
                <li>
                  <button type="button" class="row" onclick={() => at.show(phase.screen)}>
                    <span class="index">{phase.number}</span>
                    <span class="name">{phase.name}</span>
                    <span class="dot" style:background={DOT[phase.state]}></span>
                    <span class="count">{phase.met}/{phase.total}</span>
                  </button>
                </li>
              {/each}
            </ol>
          </section>
          {#each plan.slides as slide, index}
            <section
              class="screen"
              data-screen={slide.number ?? slide.name}
              hidden={viewing !== index + 1}>
              {#if slide.number !== null}
                <div class="crumb">
                  <CapsLabel>Phase {slide.number} of {plan.phases.length}</CapsLabel>
                  <span class="state">
                    <span class="dot" style:background={DOT[slide.state]}></span>
                    {WORD[slide.state]}
                  </span>
                </div>
              {/if}
              <h1 class="display-md">{slide.name}</h1>
              <div class="markdown lifted">{@html parts.sections[index].opening}</div>
              <div class="markdown">{@html parts.sections[index].body}</div>
              {#if slide.number !== null}
                <hr class="rule" />
                <div class="criteria">
                  <div class="criteria-head">
                    <CapsLabel>Acceptance criteria</CapsLabel>
                    <span class="count" data-count={slide.number}>
                      {slide.total > 0 ? `${slide.met} of ${slide.total} met` : "No checks stated"}
                    </span>
                  </div>
                  <div class="markdown">{@html parts.sections[index].criteria}</div>
                </div>
              {/if}
            </section>
          {/each}
        </div>
      {/if}
    {/snippet}
    {#snippet bar()}
      {#if live}
        <div class="bar">
          <span class="dot" style:background={allMet ? DOT.met : DOT.working}></span>
          <span class="says">
            {#if allMet}
              Every phase met
            {:else if at.following}
              Following the agent · {metered.name}
            {:else}
              Agent is on phase {metered.number} · {metered.name}
            {/if}
          </span>
          <!-- Every phase met leaves nothing to follow, so nothing is offered. -->
          {#if !allMet}
            <label class="follow">
              <input
                type="checkbox"
                checked={at.following}
                onchange={(event) => at.track(event.currentTarget.checked)} />
              Follow
            </label>
          {/if}
        </div>
      {/if}
    {/snippet}
    {#snippet controls()}
      <div class="pips">
        <button
          type="button"
          class="pip summary"
          class:viewing={viewing === 0}
          aria-label="Overview"
          aria-current={viewing === 0}
          onclick={() => reach(0)}>
          <span class="mark"></span>
        </button>
        {#each plan.slides as slide, index}
          <button
            type="button"
            class="pip"
            class:summary={slide.number === null}
            class:viewing={viewing === index + 1}
            aria-label={slide.number === null ? slide.name : `Phase ${slide.number}`}
            aria-current={viewing === index + 1}
            onclick={() => reach(index + 1)}>
            <span class="mark" style:background={slide.state ? DOT[slide.state] : null}></span>
          </button>
        {/each}
      </div>
    {/snippet}
    {#snippet counter()}
      <!-- A truncated section name is unidentifiable, so the whole of it
           stays reachable on hover. -->
      <span class="counter" title={counterFor(plan, viewing)}>{counterFor(plan, viewing)}</span>
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
            disabled={at.current === plan.slides.length}
            onclick={() => at.show(at.current + 1)}>→</Button>
        </div>
      {/if}
    {/snippet}
  </ArtifactPane>
{/if}

<style>
  /* The footer spans the pane, so the padding belongs to what it frames. */
  .deck,
  .reading {
    padding-left: var(--pane-pad);
    padding-right: var(--pane-pad);
  }

  .deck,
  .reading {
    padding-bottom: 26px;
  }

  .reading .display-lg {
    margin-top: 4px;
  }

  /* The opening screen is the one everybody sees first, so it is given room:
     label, title, lead, then the list with air around it. */
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

  .hero .lead :global(p:first-of-type) {
    color: var(--text-primary);
  }

  .hero .rows {
    margin-top: 34px;
  }

  .bar {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 8px var(--pane-pad);
    border-bottom: var(--border-width) solid var(--border-subtle);
    font: var(--machine-sm);
    color: var(--text-secondary);
  }

  .follow {
    display: inline-flex;
    align-items: center;
    gap: 8px;
    margin-left: auto;
    cursor: pointer;
  }

  /* The acceptance criteria's box, in the colour of something the reader
     operates rather than the colour of a check the document has met. */
  .follow input {
    appearance: none;
    width: 13px;
    height: 13px;
    margin: 0;
    border: var(--border-width) solid var(--border-default);
    background: transparent;
    cursor: pointer;
  }

  .follow input:checked {
    border: none;
    background: var(--accent-action);
  }

  /* The strip is a position indicator: a second row of pips would misstate the
     shape of the document, so it neither wraps nor gives up width. */
  .pips {
    display: flex;
    flex: none;
    flex-wrap: nowrap;
    gap: 6px;
  }

  .pip {
    padding: 6px 3px;
    border: 0;
    border-bottom: 2px solid transparent;
    background: transparent;
    cursor: pointer;
  }

  .pip.viewing {
    border-bottom-color: var(--accent-action);
  }

  .pip .mark {
    display: block;
    width: 14px;
    height: 5px;
  }

  .pip.summary .mark {
    box-shadow: inset 0 0 0 1px var(--text-faint);
  }

  .pip.summary {
    margin-right: 4px;
  }

  :global(.document > .footer .modes) {
    margin-left: 0;
  }

  /* Whatever width the fixed controls leave, on one line. A section's name is
     the only label here long enough to want more, and it may not have it. */
  .steps {
    display: flex;
    gap: 6px;
  }

  .screen > :global(.caps),
  .crumb {
    padding-left: var(--gutter);
  }

  .crumb {
    display: flex;
    align-items: center;
    gap: 12px;
  }

  .state {
    display: inline-flex;
    align-items: center;
    gap: 7px;
    font: var(--machine-sm);
    color: var(--text-muted);
  }

  .dot {
    display: inline-block;
    flex: none;
    width: 9px;
    height: 9px;
  }

  .display-lg,
  .display-md {
    margin: 12px 0 18px;
    padding-left: var(--gutter);
    letter-spacing: var(--display-tracking);
    color: var(--text-primary);
  }

  .display-lg {
    font: var(--display-lg);
  }

  .display-md {
    font: var(--display-md);
  }

  /* The scroller ends where the footer begins, so its bar stops there too rather
     than running the pane's full height with the footer floating over it. */
  .deck,
  .reading {
    flex: 1;
    min-height: 0;
    overflow-y: auto;
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
    min-width: 2ch;
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

  .count {
    font: var(--machine-sm);
    color: var(--text-muted);
  }

  .rule {
    border: 0;
    border-top: var(--border-width) solid var(--border-subtle);
    margin: 26px 0 18px;
  }

  .criteria-head {
    display: flex;
    align-items: baseline;
    justify-content: space-between;
    gap: 12px;
    padding-left: var(--gutter);
  }

  /* The pane names both already. Hidden but still measurable: the viewport
     reports only blocks with a box, and a span may be aimed at either line. */
  .lifted,
  .criteria .markdown :global(h3) {
    position: absolute;
    width: 1px;
    height: 1px;
    margin: 0;
    padding: 0;
    overflow: hidden;
    white-space: nowrap;
  }

  .criteria .markdown :global(li:not(:has(input:checked))) {
    color: var(--text-muted);
  }

  /* Narrow steps the type down. Nothing goes away. */
  @media (max-width: 420px) {
    .hero {
      padding-top: 6px;
    }

    /* The list is what the screen is for; the hero gives way to it. */
    .hero .display-lg {
      margin: 8px 0 12px;
    }

    .hero .rows {
      margin-top: 18px;
    }

    .hero .lead :global(p) {
      font: var(--machine-md);
    }

    .display-lg {
      font: var(--display-md);
    }

    .display-md {
      font: var(--display-sm);
    }

    .criteria .markdown :global(li) {
      font: var(--machine-xs);
    }

    /* Squeezed into the same row the pips wrap one per line, so they take a
       row of their own and the controls sit under them. */
    :global(.document > .footer .controls) {
      flex-wrap: wrap;
      row-gap: 10px;
      padding-top: 8px;
      padding-bottom: 8px;
    }

    .pips {
      flex: 1 0 100%;
    }
  }
</style>
