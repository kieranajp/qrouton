<script>
  import Button from "../core/Button.svelte";
  import CapsLabel from "../core/CapsLabel.svelte";
  import CubeMark from "../core/CubeMark.svelte";
  import { untrack } from "svelte";
  import { artifactTone } from "../artifacts.js";
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

  /** Screen 0 is the overview; phase at index n is screen n + 1. */
  let current = $state(untrack(() => screenFor(plan.phases, doc.line ?? 0)));
  /** @type {HTMLElement | undefined} */
  let body = $state();

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

  // The pipeline renders the document once; the deck is that markup dealt out by
  // the source lines it already carries. A block the parser placed nowhere — a
  // list container, say — takes the range of the numbered blocks inside it, and
  // one carrying no line at all follows whichever block came before it.
  function partition(html, parsed) {
    const holder = document.createElement("div");
    holder.innerHTML = html;
    const preamble = [];
    const phases = parsed.phases.map(() => ({ body: [], criteria: [] }));
    let at = { from: 0, to: 0 };
    for (const node of [...holder.children]) {
      at = spanOf(node) ?? at;
      const index = parsed.phases.findIndex((phase) => at.from >= phase.from && at.from <= phase.to);
      if (index < 0) {
        preamble.push(node.outerHTML);
        continue;
      }
      const verify = criteriaSpans(parsed.phases[index]);
      const lifted = verify && at.from >= verify.from && at.to <= verify.to;
      phases[index][lifted ? "criteria" : "body"].push(node.outerHTML);
    }
    return {
      preamble: preamble.join(""),
      phases: phases.map((phase) => ({
        body: phase.body.join(""),
        criteria: phase.criteria.join(""),
      })),
    };
  }

  // A mark answers one open_file request, so any navigation the reader makes
  // retires it rather than leaving marks strewn across the screens.
  function show(screen) {
    for (const marked of body?.querySelectorAll(".marked") ?? []) marked.classList.remove("marked");
    current = Math.max(0, Math.min(screen, plan.phases.length));
  }

  /** @param {KeyboardEvent} event */
  function onKey(event) {
    if (!active || event.metaKey || event.ctrlKey || event.altKey) return;
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
  function diagrams(deckBody) {
    const off = Events.On("window:diagram:" + id, (event) => applyDiagrams(deckBody, [event.data]));
    Call.ByName(WINDOWS_SERVICE + ".RenderDiagrams", id)
      .then((found) => applyDiagrams(deckBody, found ?? []))
      .catch(() => {});
    return {
      destroy: () => {
        off();
        teardownDiagrams(deckBody);
      },
    };
  }

  // The request is answered on the screen it opens, and no further: a span
  // running past a phase boundary says nothing about the phase after it, so the
  // pane neither marks that part nor scrolls to it.
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
          blocks,
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
      // A screen change is a visibility change, and the hidden blocks measure as
      // nothing; it is never a reason to scroll to the request's target again.
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
        <span>{heading}</span>
      </div>
    {/if}
    <div
      class="deck"
      bind:this={body}
      data-document-source={doc.source}
      use:links
      use:diagrams
      use:viewport={{ id, active, scrollRoot, screen: current }}>
      <section class="screen" data-screen="overview" hidden={current !== 0}>
        <CapsLabel
          >Plan · {plan.phases.length}
          {plan.phases.length === 1 ? "phase" : "phases"}</CapsLabel>
        <h1 class="display-lg">{plan.title || heading}</h1>
        <div class="markdown">{@html deck.preamble}</div>
        <ol class="rows">
          {#each plan.phases as phase, at (phase.index)}
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
      {#each plan.phases as phase, at (phase.index)}
        <section class="screen" data-screen={phase.index} hidden={current !== at + 1}>
          <div class="crumb">
            <CapsLabel>Phase {phase.index} of {plan.phases.length}</CapsLabel>
            <span class="state">
              <span class="dot" style:background={DOT[phase.state]}></span>
              {WORD[phase.state]}
            </span>
          </div>
          <h1 class="display-md">{phase.name}</h1>
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

  /* The document's own Verify heading is left in the flow, out of sight: the
     viewport reports the blocks it can measure, and display:none would drop the
     one an open_file span aimed at this list would land on. */
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
</style>
