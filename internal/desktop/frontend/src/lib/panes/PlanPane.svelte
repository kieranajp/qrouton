<script>
  import Button from "../core/Button.svelte";
  import CapsLabel from "../core/CapsLabel.svelte";
  import Chip from "../core/Chip.svelte";
  import CubeMark from "../core/CubeMark.svelte";
  import { untrack } from "svelte";
  import { artifactTone } from "../artifacts.js";
  import { chrome } from "../chrome.svelte.js";
  import { Call, copyText } from "../wails.js";
  import { diagrams, links } from "./actions.js";
  import MarkdownPane from "./MarkdownPane.svelte";
  import { marks, render } from "./markdown.js";
  import { criteriaSpans, parsePlan } from "./plan.js";
  import { dealt } from "./sections.js";
  import { createViewportController, nextViewportSequence } from "./viewport.js";
  import "./markdown.css";

  const WINDOWS_SERVICE = "github.com/kieranajp/qrouton/internal/desktop.Windows";

  const DOT = {
    met: "var(--state-success)",
    working: "var(--state-running)",
    "not-started": "var(--text-faint)",
  };
  const WORD = { met: "Met", working: "Working", "not-started": "Not started" };

  /** @type {{doc: {text: string, format: string, source: string, path?: string, kind?: string, line?: number, to?: number, viewportEpoch?: number}, id: string, active?: boolean, scrollRoot?: HTMLElement}} */
  let { doc, id, active = false, scrollRoot } = $props();

  let rendered = $derived(render(doc.text));
  let plan = $derived(parsePlan(doc.text));
  let deck = $derived(partition(rendered.body, plan));
  let heading = $derived(rendered.title || (doc.source ? doc.source.split("/").pop() : ""));
  let tone = $derived(artifactTone(doc.kind));
  let copied = $state(false);
  let mode = $state("plan");
  let pinned = $state(false);
  // A mark answers the request that opened the pane; a remount must not revive
  // one the reader has already navigated away from.
  let retired = $state(false);

  const session = chrome();
  let allMet = $derived(plan.phases.length > 0 && plan.phases.every((phase) => phase.state === "met"));
  // The phase the meter rests on, and with nothing unmet left to point at, the
  // last one. The bar names it from the phase itself: a screen counts sections
  // too, so a screen number is not a phase number and cannot index the phases.
  let metered = $derived(plan.phases.find((phase) => phase.state !== "met") ?? plan.phases.at(-1));
  let followed = $derived(metered?.screen ?? 0);
  // An agent is working somewhere in this session. Nothing here knows whether
  // it is working on this plan, and the bar must not say that it does. A deck
  // of nothing but sections has no meter, so it has nothing to report.
  let live = $derived((session.fields.activity === "working" || allMet) && Boolean(metered));

  /** Screen 0 is the overview; phase at index n is screen n + 1. */
  let current = $state(untrack(() => screenFor(plan.slides, doc.line ?? 0)));
  // The pane is on the meter's phase and will stay with it. Anything that moves
  // the reader off it, or pins them to it, ends that.
  let following = $derived(!pinned && followed === current);
  /** @type {HTMLElement | undefined} */
  let body = $state();
  /** @type {HTMLElement | undefined} */
  let reading = $state();
  let epoch = untrack(() => doc.viewportEpoch);

  // A push carries the span along with the text, so only a reload — which is
  // what moves the epoch — counts as a fresh request to jump.
  $effect(() => {
    const at = doc.viewportEpoch;
    const count = plan.slides.length;
    untrack(() => {
      if (at !== epoch) {
        epoch = at;
        current = screenFor(plan.slides, doc.line ?? 0);
        pinned = false;
        retired = false;
      }
      if (current > count) current = count;
    });
  });

  function screenFor(slides, line) {
    if (!line || line < 1) return 0;
    const at = slides.findIndex((slide) => line >= slide.from && line <= slide.to);
    return at < 0 ? 0 : at + 1;
  }

  // A phase slide counts in phases, because that is what its heading numbers.
  // Anything else answers with its own name, which is the only honest label a
  // section has: it has no position in a sequence the document defines.
  function counterFor(screen) {
    if (screen === 0) return "Overview";
    const slide = plan.slides[screen - 1];
    return slide.number === null ? slide.name : `${slide.number} / ${plan.phases.length}`;
  }

  // The deck is one rendered document dealt out by the source lines its blocks
  // already carry: the opening heading, the body, and the criteria the phase
  // states, each into the slide whose span holds it.
  function partition(html, parsed) {
    const preamble = [];
    const slides = parsed.slides.map(() => ({ opening: [], body: [], criteria: [] }));
    for (const block of dealt(html)) {
      const index = parsed.slides.findIndex(
        (slide) => block.from >= slide.from && block.from <= slide.to,
      );
      if (index < 0) {
        preamble.push(block.html);
        continue;
      }
      const verify = criteriaSpans(parsed.slides[index]);
      const bucket =
        block.from === parsed.slides[index].from
          ? "opening"
          : verify && block.from >= verify.from && block.to <= verify.to
            ? "criteria"
            : "body";
      slides[index][bucket].push(block.html);
    }
    return {
      preamble: preamble.join(""),
      slides: slides.map((slide) => ({
        opening: slide.opening.join(""),
        body: slide.body.join(""),
        criteria: slide.criteria.join(""),
      })),
    };
  }

  // A mark answers one open_file request, so navigating retires it. Every
  // control pins the reader; the bar's Follow button hands the position back.
  function show(screen, pin = true) {
    for (const marked of body?.querySelectorAll(".marked") ?? []) marked.classList.remove("marked");
    retired = true;
    pinned = pin;
    current = Math.max(0, Math.min(screen, plan.slides.length));
  }

  // The slide on screen, and the one the footer names: one value, so they
  // cannot drift apart. In Document mode nothing is hidden, so it is the slide
  // the reader has scrolled into rather than the one they selected.
  let scrolled = $state(0);
  let viewing = $derived(mode === "document" ? scrolled : current);

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
    if (mode === "plan") {
      show(screen);
      return;
    }
    const from = screen === 0 ? plan.preamble.from : plan.slides[screen - 1].from;
    const blocks = [
      .../** @type {NodeListOf<HTMLElement>} */ (reading?.querySelectorAll("[data-line]") ?? []),
    ];
    const target = blocks.find((block) => Number(block.dataset.line) >= from) ?? blocks[0];
    target?.scrollIntoView({ block: "start" });
  }

  // Following moves the view when the meter moves. It does not choose the
  // screen a document opens on: that is the overview, or the phase a span asked
  // for, and neither is the agent's to overrule.
  let meter = untrack(() => followed);
  $effect(() => {
    const to = followed;
    untrack(() => {
      if (to === meter) return;
      meter = to;
      if (live && !pinned) current = to;
    });
  });

  // The tick is the reader's grip on the meter. Taking it up moves them to the
  // phase the meter is on; letting it go leaves them exactly where they are.
  function track(on) {
    if (on) show(followed, false);
    else pinned = true;
  }

  /** @param {KeyboardEvent} event */
  function onKey(event) {
    if (!active || plan.slides.length === 0) return;
    if (event.metaKey || event.ctrlKey || event.altKey) return;
    const from = /** @type {HTMLElement} */ (event.target);
    const field = /^(input|textarea|select)$/i.test(from?.tagName ?? "");
    // A field owns the arrows because they move its caret; a tick box has none,
    // so the deck keeps them while the reader's focus rests on Follow.
    const tick = /** @type {HTMLInputElement} */ (from)?.type === "checkbox";
    if (from?.isContentEditable || (field && !tick)) return;
    if (event.key === "ArrowRight") show(current + 1);
    else if (event.key === "ArrowLeft") show(current - 1);
    else return;
    event.preventDefault();
  }

  async function copyPath() {
    if (!doc.path) return;
    try {
      await copyText(doc.path);
      copied = true;
      setTimeout(() => (copied = false), 1200);
    } catch {}
  }

  // A span running past a phase boundary says nothing about the phase after
  // it, so the pane neither marks that part nor scrolls to it.
  function requested() {
    const line = doc.line ?? 0;
    const to = doc.to ?? 0;
    const opened = current > 0 ? plan.slides[current - 1] : plan.preamble;
    return { line, to: to > line ? Math.min(to, opened.to) : to };
  }

  /** @param {HTMLElement} deckBody */
  function viewport(deckBody, initial) {
    const blocks = [
      .../** @type {NodeListOf<HTMLElement>} */ (deckBody.querySelectorAll("[data-line]")),
    ];
    const span = requested();
    const { marked, at } = marks(
      blocks.map((el) => ({ line: Number(el.dataset.line), end: Number(el.dataset.lineEnd) })),
      span,
    );
    if (!untrack(() => retired)) for (const index of marked) blocks[index].classList.add("marked");
    const target = blocks[at];
    let controller;
    let root;
    let windowID;
    let screen;
    const apply = (params) => {
      if (!params.scrollRoot) return;
      if (!controller || root !== params.scrollRoot || windowID !== params.id) {
        controller?.destroy();
        root = params.scrollRoot;
        windowID = params.id;
        screen = params.screen;
        controller = createViewportController({
          root,
          content: deckBody,
          target,
          span,
          selected: params.active,
          nextSequence: () => nextViewportSequence(windowID),
          report: (report) =>
            Call.ByName(WINDOWS_SERVICE + ".ReportViewport", windowID, {
              epoch: doc.viewportEpoch,
              ...report,
            }).catch(() => {}),
        });
        return;
      }
      controller.setSelected(params.active);
      // Changing screens changes what can be measured, never where to scroll.
      if (screen !== params.screen) {
        screen = params.screen;
        controller.schedule();
      }
    };
    apply(initial);
    return {
      update: apply,
      destroy: () => controller?.destroy(),
    };
  }
</script>

<svelte:window onkeydown={onKey} />

{#if plan.slides.length === 0}
  <MarkdownPane {doc} {id} {active} {scrollRoot} />
{:else}
  <article class="document plan">
    <div class="head">
      <CubeMark size={18} face={tone} data-artifact-kind={doc.kind ?? "NOTE"} />
      <Chip>{doc.kind ?? "PLAN"}</Chip>
      {#if doc.source}
        <CapsLabel tone="dim">{doc.source}</CapsLabel>
      {/if}
      {#if doc.path}
        <Button
          variant="ghost"
          size="sm"
          aria-label="Copy absolute path"
          title={doc.path}
          onclick={copyPath}>{copied ? "Copied" : "Copy"}</Button>
      {/if}
    </div>
    {#if mode === "document"}
      <div class="reading" bind:this={reading}>
        <!-- The renderer lifts the opening heading out of the body, so the
             document view states the plan's name itself. -->
        <h1 class="display-lg">{plan.title || heading}</h1>
        <MarkdownPane {doc} {id} {active} {scrollRoot} bare onMeasure={spy} />
      </div>
    {:else}
      <div
        class="deck"
        bind:this={body}
        data-document-source={doc.source}
        use:links={doc.source}
        use:diagrams={{ id, text: doc.text }}
        use:viewport={{ id, active, scrollRoot, screen: current }}>
        <section class="screen hero" data-screen="overview" hidden={viewing !== 0}>
          <CapsLabel
            >Plan · {plan.phases.length}
            {plan.phases.length === 1 ? "phase" : "phases"}</CapsLabel>
          <h1 class="display-lg">{plan.title || heading}</h1>
          <div class="markdown lead">{@html deck.preamble}</div>
          <ol class="rows">
            {#each plan.phases as phase}
              <li>
                <button type="button" class="row" onclick={() => show(phase.screen)}>
                  <span class="index">{phase.number}</span>
                  <span class="name">{phase.name}</span>
                  <span class="dot" style:background={DOT[phase.state]}></span>
                  <span class="count">{phase.met}/{phase.total}</span>
                </button>
              </li>
            {/each}
          </ol>
        </section>
        {#each plan.slides as slide, at}
          <section
            class="screen"
            data-screen={slide.number ?? slide.name}
            hidden={viewing !== at + 1}>
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
            <div class="markdown lifted">{@html deck.slides[at].opening}</div>
            <div class="markdown">{@html deck.slides[at].body}</div>
            {#if slide.number !== null}
              <hr class="rule" />
              <div class="criteria">
                <div class="criteria-head">
                  <CapsLabel>Acceptance criteria</CapsLabel>
                  <span class="count" data-count={slide.number}>
                    {slide.total > 0 ? `${slide.met} of ${slide.total} met` : "No checks stated"}
                  </span>
                </div>
                <div class="markdown">{@html deck.slides[at].criteria}</div>
              </div>
            {/if}
          </section>
        {/each}
      </div>
    {/if}
    <footer class="footer">
      {#if live}
        <div class="bar">
          <span class="dot" style:background={allMet ? DOT.met : DOT.working}></span>
          <span class="says">
            {#if allMet}
              Every phase met
            {:else if following}
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
                checked={following}
                onchange={(event) => track(event.currentTarget.checked)} />
              Follow
            </label>
          {/if}
        </div>
      {/if}
      <div class="controls">
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
          {#each plan.slides as slide, at}
            <button
              type="button"
              class="pip"
              class:summary={slide.number === null}
              class:viewing={viewing === at + 1}
              aria-label={slide.number === null ? slide.name : `Phase ${slide.number}`}
              aria-current={viewing === at + 1}
              onclick={() => reach(at + 1)}>
              <span class="mark" style:background={slide.state ? DOT[slide.state] : null}></span>
            </button>
          {/each}
        </div>
        <div class="modes">
          <Button
            variant={mode === "plan" ? "outline" : "ghost"}
            size="sm"
            aria-pressed={mode === "plan"}
            onclick={() => (mode = "plan")}>Plan</Button>
          <Button
            variant={mode === "document" ? "outline" : "ghost"}
            size="sm"
            aria-pressed={mode === "document"}
            onclick={() => (mode = "document")}>Document</Button>
        </div>
        <!-- A truncated section name is unidentifiable, so the whole of it
             stays reachable on hover. -->
        <span class="counter" title={counterFor(viewing)}>{counterFor(viewing)}</span>
        {#if mode === "plan"}
          <div class="steps">
            <Button
              variant="ghost"
              size="sm"
              aria-label="Previous screen"
              disabled={current === 0}
              onclick={() => show(current - 1)}>←</Button>
            <Button
              variant="ghost"
              size="sm"
              aria-label="Next screen"
              disabled={current === plan.slides.length}
              onclick={() => show(current + 1)}>→</Button>
          </div>
        {/if}
      </div>
    </footer>
  </article>
{/if}

<style>
  .document {
    --pane-pad: 34px;
    display: flex;
    flex-direction: column;
    flex: 1;
    min-height: 0;
  }

  /* The footer spans the pane, so the padding belongs to what it frames. */
  .deck,
  .reading {
    padding-left: var(--pane-pad);
    padding-right: var(--pane-pad);
  }

  /* Aligned with the body's text column rather than the pane edge, so the
     mark, the chip and the path sit over the deck's own left margin. */
  .head {
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 26px var(--pane-pad) 20px calc(var(--pane-pad) + var(--gutter));
  }

  .head :global(.caps) {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .deck,
  .reading {
    padding-bottom: 26px;
  }

  .reading .display-lg {
    margin-top: 4px;
  }

  .modes {
    display: flex;
    gap: 6px;
    margin-left: auto;
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

  /* Held on the pane's floor whatever the phase is tall enough to fill, so the
     arrows and pips stay under the same finger from one screen to the next. */
  .footer {
    position: sticky;
    bottom: 0;
    margin-top: auto;
    background: var(--surface-chrome);
    border-top: var(--border-width) solid var(--border-subtle);
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

  .controls {
    display: flex;
    align-items: center;
    gap: 14px;
    min-height: var(--h-footer);
    padding: 0 var(--pane-pad);
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

  .footer .modes {
    margin-left: 0;
  }

  /* Whatever width the fixed controls leave, on one line. A section's name is
     the only label here long enough to want more, and it may not have it. */
  .counter {
    flex: 1 1 0;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    text-align: right;
    font: var(--machine-sm);
    color: var(--text-muted);
  }

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

  .deck,
  .reading {
    flex: 1;
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
    .controls {
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
