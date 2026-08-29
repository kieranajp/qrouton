<script>
  import Button from "../core/Button.svelte";
  import CapsLabel from "../core/CapsLabel.svelte";
  import Chip from "../core/Chip.svelte";
  import CubeMark from "../core/CubeMark.svelte";
  import { untrack } from "svelte";
  import { artifactTone } from "../artifacts.js";
  import { chrome } from "../chrome.svelte.js";
  import { openDocument } from "../docked.svelte.js";
  import { Call, Events, copyText, openURL } from "../wails.js";
  import { apply as applyDiagrams, teardown as teardownDiagrams } from "./diagrams.js";
  import MarkdownPane from "./MarkdownPane.svelte";
  import { documentPath, linkKind, marks, render } from "./markdown.js";
  import { criteriaSpans, parsePlan } from "./plan.js";
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
  let raw = $state(false);
  // A span is a direct request for a phase, so it holds the reader there until
  // they hand the position back. Following only ever moves an unasked-for view.
  let pinned = $state(untrack(() => (doc.line ?? 0) > 0));

  const session = chrome();
  let allMet = $derived(plan.phases.length > 0 && plan.phases.every((phase) => phase.state === "met"));
  // With nothing unmet left to point at, the meter rests on the last phase.
  let followed = $derived.by(() => {
    const at = plan.phases.findIndex((phase) => phase.state !== "met");
    return at < 0 ? plan.phases.length : at + 1;
  });
  // An agent is working somewhere in this session. Nothing here knows whether
  // it is working on this plan, and the bar must not say that it does.
  let live = $derived(session.fields.activity === "working" || allMet);

  /** Screen 0 is the overview; phase at index n is screen n + 1. */
  let current = $state(untrack(() => screenFor(plan.phases, doc.line ?? 0)));
  /** @type {HTMLElement | undefined} */
  let body = $state();
  let epoch = untrack(() => doc.viewportEpoch);

  // A push carries the span along with the text, so only a reload — which is
  // what moves the epoch — counts as a fresh request to jump.
  $effect(() => {
    const at = doc.viewportEpoch;
    const count = plan.phases.length;
    untrack(() => {
      if (at !== epoch) {
        epoch = at;
        current = screenFor(plan.phases, doc.line ?? 0);
        pinned = (doc.line ?? 0) > 0;
      }
      if (current > count) current = count;
    });
  });

  function screenFor(phases, line) {
    if (!line || line < 1) return 0;
    const at = phases.findIndex((phase) => line >= phase.from && line <= phase.to);
    return at < 0 ? 0 : at + 1;
  }

  /** @param {Element} node */
  function spanOf(node) {
    const own = Number(/** @type {HTMLElement} */ (node).dataset?.line);
    if (own > 0) {
      return { from: own, to: Number(/** @type {HTMLElement} */ (node).dataset.lineEnd) || own };
    }
    const inside = [...node.querySelectorAll("[data-line]")].map((el) => ({
      from: Number(/** @type {HTMLElement} */ (el).dataset.line),
      to: Number(/** @type {HTMLElement} */ (el).dataset.lineEnd),
    }));
    if (inside.length === 0) return null;
    return {
      from: Math.min(...inside.map((at) => at.from)),
      to: Math.max(...inside.map((at) => at.to || at.from)),
    };
  }

  // The deck is one rendered document dealt out by the source lines its blocks
  // already carry. A block the parser numbered nowhere takes the range of the
  // numbered blocks inside it, or failing that the range of the block before it.
  function partition(html, parsed) {
    const holder = document.createElement("div");
    holder.innerHTML = html;
    const preamble = [];
    const phases = parsed.phases.map(() => ({ opening: [], body: [], criteria: [] }));
    let at = { from: 0, to: 0 };
    for (const node of [...holder.children]) {
      at = spanOf(node) ?? at;
      const index = parsed.phases.findIndex((phase) => at.from >= phase.from && at.from <= phase.to);
      if (index < 0) {
        preamble.push(node.outerHTML);
        continue;
      }
      const verify = criteriaSpans(parsed.phases[index]);
      const bucket =
        at.from === parsed.phases[index].from
          ? "opening"
          : verify && at.from >= verify.from && at.to <= verify.to
            ? "criteria"
            : "body";
      phases[index][bucket].push(node.outerHTML);
    }
    return {
      preamble: preamble.join(""),
      phases: phases.map((phase) => ({
        opening: phase.opening.join(""),
        body: phase.body.join(""),
        criteria: phase.criteria.join(""),
      })),
    };
  }

  // A mark answers one open_file request, so navigating retires it. Every
  // control pins the reader; the bar's Follow button hands the position back.
  function show(screen, pin = true) {
    for (const marked of body?.querySelectorAll(".marked") ?? []) marked.classList.remove("marked");
    pinned = pin;
    current = Math.max(0, Math.min(screen, plan.phases.length));
  }

  $effect(() => {
    const to = followed;
    const following = live && !pinned;
    untrack(() => {
      if (following) current = to;
    });
  });

  /** @param {KeyboardEvent} event */
  function onKey(event) {
    if (!active || plan.phases.length === 0) return;
    if (event.metaKey || event.ctrlKey || event.altKey) return;
    const from = /** @type {HTMLElement} */ (event.target);
    if (from?.isContentEditable || /^(input|textarea|select)$/i.test(from?.tagName ?? "")) return;
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

  /** @param {HTMLElement} deckBody */
  function links(deckBody) {
    /** @param {MouseEvent} event */
    const click = (event) => {
      const anchor = /** @type {HTMLElement} */ (event.target)?.closest("a");
      if (!anchor) return;
      const href = anchor.getAttribute("href");
      event.preventDefault();
      if (linkKind(href) === "document") {
        openDocument(documentPath(href ?? "", doc.source)).catch(() => {});
      } else if (linkKind(href) === "external") {
        openURL(href ?? "");
      }
    };
    deckBody.addEventListener("click", click);
    return { destroy: () => deckBody.removeEventListener("click", click) };
  }

  /** @param {HTMLElement} deckBody */
  function diagrams(deckBody, _text) {
    const off = Events.On("window:diagram:" + id, (event) => applyDiagrams(deckBody, [event.data]));
    // Rendered markup does not survive a content push, so the fences are asked
    // for again whenever the text behind them changes.
    const draw = () =>
      Call.ByName(WINDOWS_SERVICE + ".RenderDiagrams", id)
        .then((found) => applyDiagrams(deckBody, found ?? []))
        .catch(() => {});
    draw();
    return {
      update: draw,
      destroy: () => {
        off();
        teardownDiagrams(deckBody);
      },
    };
  }

  // A span running past a phase boundary says nothing about the phase after
  // it, so the pane neither marks that part nor scrolls to it.
  function requested() {
    const line = doc.line ?? 0;
    const to = doc.to ?? 0;
    const opened = current > 0 ? plan.phases[current - 1] : plan.preamble;
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
    for (const index of marked) blocks[index].classList.add("marked");
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

{#if plan.phases.length === 0}
  <MarkdownPane {doc} {id} {active} {scrollRoot} />
{:else}
  <article class="document plan">
    {#if doc.source}
      <div class="source">
        <CapsLabel tone="dim">{doc.source}</CapsLabel>
        {#if doc.path}
          <Button
            variant="ghost"
            size="sm"
            aria-label="Copy absolute path"
            title={doc.path}
            onclick={copyPath}>{copied ? "Copied" : "Copy"}</Button>
        {/if}
      </div>
    {/if}
    {#if heading}
      <div class="title">
        <CubeMark size={18} face={tone} data-artifact-kind={doc.kind ?? "NOTE"} />
        <Chip>{doc.kind ?? "PLAN"}</Chip>
        <span class="name">{heading}</span>
        <div class="modes">
          <Button
            variant={raw ? "ghost" : "outline"}
            size="sm"
            aria-pressed={!raw}
            onclick={() => (raw = false)}>Plan</Button>
          <Button
            variant={raw ? "outline" : "ghost"}
            size="sm"
            aria-pressed={raw}
            onclick={() => (raw = true)}>Raw</Button>
        </div>
      </div>
    {/if}
    <div
      class="deck"
      hidden={raw}
      bind:this={body}
      data-document-source={doc.source}
      use:links
      use:diagrams={doc.text}
      use:viewport={{ id, active, scrollRoot, screen: current }}>
      <section class="screen" data-screen="overview" hidden={current !== 0}>
        <CapsLabel
          >Plan · {plan.phases.length}
          {plan.phases.length === 1 ? "phase" : "phases"}</CapsLabel>
        <h1 class="display-lg">{plan.title || heading}</h1>
        <div class="markdown">{@html deck.preamble}</div>
        <ol class="rows">
          {#each plan.phases as phase, at}
            <li>
              <button type="button" class="row" onclick={() => show(at + 1)}>
                <span class="index">{phase.index}</span>
                <span class="name">{phase.name}</span>
                <span class="dot" style:background={DOT[phase.state]}></span>
                <span class="count">{phase.met}/{phase.total}</span>
              </button>
            </li>
          {/each}
        </ol>
      </section>
      {#each plan.phases as phase, at}
        <section class="screen" data-screen={phase.index} hidden={current !== at + 1}>
          <div class="crumb">
            <CapsLabel>Phase {at + 1} of {plan.phases.length}</CapsLabel>
            <span class="state">
              <span class="dot" style:background={DOT[phase.state]}></span>
              {WORD[phase.state]}
            </span>
          </div>
          <h1 class="display-md">{phase.name}</h1>
          <div class="markdown lifted">{@html deck.phases[at].opening}</div>
          <div class="markdown">{@html deck.phases[at].body}</div>
          <hr class="rule" />
          <div class="criteria">
            <div class="criteria-head">
              <CapsLabel>Acceptance criteria</CapsLabel>
              <span class="count" data-count={phase.index}>
                {phase.total > 0 ? `${phase.met} of ${phase.total} met` : "No checks stated"}
              </span>
            </div>
            <div class="markdown">{@html deck.phases[at].criteria}</div>
          </div>
        </section>
      {/each}
    </div>
    <pre class="raw" hidden={!raw}>{doc.text}</pre>
    <footer class="footer">
      {#if live}
        {@const moved = pinned && followed !== current}
        <div class="bar">
          <span class="dot" style:background={allMet ? DOT.met : DOT.working}></span>
          <span class="says">
            {#if allMet}
              Every phase met
            {:else if moved}
              Agent moved to phase {followed} · {plan.phases[followed - 1].name}
            {:else}
              Following the agent · {plan.phases[followed - 1].name}
            {/if}
          </span>
          {#if moved}
            <Button variant="ghost" size="sm" onclick={() => show(followed, false)}>Follow</Button>
          {/if}
        </div>
      {/if}
      <div class="controls">
        <div class="pips">
          {#each plan.phases as phase, at}
            <button
              type="button"
              class="pip"
              class:viewing={current === at + 1}
              aria-label="Phase {phase.index}"
              aria-current={current === at + 1}
              onclick={() => show(at + 1)}>
              <span class="mark" style:background={DOT[phase.state]}></span>
            </button>
          {/each}
        </div>
        <span class="counter">
          {current === 0 ? "Overview" : `${current} / ${plan.phases.length}`}
        </span>
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
            disabled={current === plan.phases.length}
            onclick={() => show(current + 1)}>→</Button>
        </div>
      </div>
    </footer>
  </article>
{/if}

<style>
  .document {
    padding: 26px 34px;
    --gutter: 4.5ch;
  }

  .source,
  .title {
    padding-left: var(--gutter);
  }

  .source {
    display: flex;
    align-items: center;
    gap: 9px;
  }

  .source :global(.caps) {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .title {
    display: flex;
    align-items: center;
    gap: 12px;
    margin: 10px 0 18px;
    font: var(--display-sm);
    letter-spacing: var(--display-tracking);
    color: var(--text-primary);
  }

  .title .name {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .modes {
    display: flex;
    gap: 6px;
    margin-left: auto;
  }

  .raw {
    margin: 0;
    padding: 18px var(--gutter);
    background: var(--surface-terminal);
    border: var(--border-width) solid var(--border-subtle);
    font: var(--terminal);
    color: var(--text-secondary);
    white-space: pre-wrap;
  }

  .footer {
    position: sticky;
    bottom: 0;
    margin-top: 26px;
    background: var(--surface-chrome);
    border-top: var(--border-width) solid var(--border-subtle);
  }

  .bar {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 8px var(--space-3);
    border-bottom: var(--border-width) solid var(--border-subtle);
    font: var(--machine-sm);
    color: var(--text-secondary);
  }

  .controls {
    display: flex;
    align-items: center;
    gap: 14px;
    min-height: var(--h-footer);
    padding: 0 var(--space-3);
  }

  .pips {
    display: flex;
    flex-wrap: wrap;
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

  .counter {
    margin-left: auto;
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

  .rows {
    list-style: none;
    margin: 26px 0 0;
    padding: 0;
    border: var(--border-width) solid var(--border-subtle);
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
    .display-lg {
      font: var(--display-md);
    }

    .display-md {
      font: var(--display-sm);
    }

    .criteria .markdown :global(li) {
      font: var(--machine-xs);
    }
  }
</style>
