<script>
  import { ARTIFACT_KINDS } from "../bridge/generated.js";
  import ArtifactTag from "../core/ArtifactTag.svelte";
  import Button from "../core/Button.svelte";
  import CapsLabel from "../core/CapsLabel.svelte";
  import Checkbox from "../core/Checkbox.svelte";
  import Chip from "../core/Chip.svelte";
  import Icon from "../core/Icon.svelte";
  import { ICON_COLOURS } from "../core/icons.js";
  import { dashboard } from "../dashboard.svelte.js";
  import { STATE_WORDS } from "../dashboard/dashboard.js";
  import { ICONS } from "../overview/pulls.js";
  import { copyText, openURL } from "../wails.js";
  import ArtifactPane from "./ArtifactPane.svelte";
  import CopyCode from "./CopyCode.svelte";
  import { links, viewport } from "./actions.js";
  import { answerable } from "./answerable.svelte.js";
  import { clampedSpan, holding } from "./deck.js";
  import MarkdownPane from "./MarkdownPane.svelte";
  import { render } from "./markdown.js";
  import { reader, scrolls } from "./reader.svelte.js";
  import { dealt } from "./sections.js";
  import { sendTodoUpdate } from "./spec-calls.js";
  import { parseTodo, pullRefs, reapply, runCommand, spliceDraft, todoKey } from "./todo.js";
  import "./markdown.css";

  /** @type {{doc: {text: string, hash?: string, format: string, source: string, path?: string, kind?: string, line?: number, to?: number, viewportEpoch?: number}, id: string, slug?: string, active?: boolean, scrollRoot?: HTMLElement, onScroller?: (element: HTMLElement | null) => void}} */
  let { doc, id, slug = "", active = false, scrollRoot, onScroller } = $props();

  /** @typedef {import("./todo.js").Draft} Draft */
  /** @typedef {import("./todo.js").Item} Item */
  /** @typedef {import("./todo.js").State} State */

  const STATES = { unsaved: "Not saved", saving: "Saving…", saved: "Saved", failed: "Not saved", reloaded: "Not saved" };
  const BOXES = { open: "[ ]", done: "[x]", skipped: "[-]" };

  const answers = answerable({
    id: () => id,
    doc: () => doc,
    splice,
    reloaded: "The list changed on disk, so it reloaded. Your change is kept here: save it again.",
  });
  const { pending, saves } = answers;
  const pulls = dashboard();

  /** @param {string} text @param {string} _key @param {Draft} draft */
  function splice(text, _key, draft) {
    return spliceDraft(text, draft);
  }

  let source = $derived(answers.source);
  let shown = $derived({ ...doc, text: source.text, viewportEpoch: answers.epoch });
  let rendered = $derived(render(source.text));
  let todo = $derived(parseTodo(source.text));
  let laid = $derived(reapply(todo, pending));
  let heading = $derived(todo.title || (doc.source ? doc.source.split("/").pop() : ""));

  /** @type {{item: Item, key: string, state: State, note: string}[]} */
  let rows = $derived(
    laid.items.map(({ item, draft }) => ({ item, key: todoKey(item), state: draft?.state ?? item.state, note: draft?.note ?? item.note })),
  );
  let next = $derived(rows.find((row) => row.state === "open")?.key ?? "");
  /** @type {import("../dashboard/dashboard.js").PullRequest[]} */
  let tracked = $derived(pulls.status.sessions?.[slug]?.pullRequests ?? []);

  /** @param {{state: State}[]} of */
  const tally = (of) => ({ done: of.filter((row) => row.state === "done").length, total: of.filter((row) => row.state !== "skipped").length });
  let overall = $derived(tally(rows));
  let blocking = $derived(rows.filter((row) => row.state === "open" && row.item.blocks).length);

  let blocks = $derived(dealt(rendered.body));
  let groups = $derived(
    todo.sections
      .map((section) => {
        const held = rows.filter((row) => row.item.line >= section.from && row.item.line <= section.to);
        const prose = blocks
          .filter((block) => block.from >= section.from && block.from <= section.to && block.from !== (section.name ? section.from : 0))
          .filter((block) => !todo.items.some((item) => item.line >= block.from && item.line <= block.to))
          .map((block) => block.html)
          .join("");
        return { ...section, rows: held, prose, ...tally(held) };
      })
      .filter((group) => group.name || group.rows.length || group.prose),
  );

  /** @param {{item: Item, state: State, note: string}} row @param {Partial<{state: State, note: string}>} change */
  function edit(row, change, typed = false) {
    sent = null;
    const key = todoKey(row.item);
    /** @type {Draft} */
    // A note rides along only once typed, so a tick keeps whatever note is on disk when it saves.
    const draft = { ...pending.get(key), key, section: row.item.section, text: row.item.text, state: row.state, ...change };
    if (typed) answers.type(key, draft);
    else answers.put(key, draft);
  }

  /** @param {{item: Item, state: State, note: string}} row @param {State} state */
  function mark(row, state) {
    edit(row, { state });
    answers.commit(todoKey(row.item));
  }

  /** @param {Draft} draft */
  const written = (draft) => `- ${BOXES[draft.state]} ${draft.text}${draft.note ? ` — ${draft.note}` : ""}`;

  let blocked = $derived.by(() => {
    const waiting = answers.waiting;
    if (waiting === "saving") return "Wait for the save to finish.";
    if (waiting === "failed") return "A change did not save. Save it again first.";
    if (laid.detached.length > 0) return "Copy or discard the detached changes first.";
    if (waiting === "typed") return "Save your typed note first: click outside the field or press ⌘↵.";
    return "";
  });
  let sent = $state(/** @type {{ok: boolean, message: string} | null} */ (null));

  async function send() {
    if (blocked) return;
    const typed = await sendTodoUpdate(id);
    sent = typed.ok
      ? { ok: true, message: "Typed into the conversation." }
      : { ok: false, message: `Could not type into the conversation: ${typed.error?.message ?? typed.error}` };
  }

  /** @type {HTMLElement | undefined} */
  let sheet = $state();
  /** @type {HTMLElement | undefined} */
  let reading = $state();

  const view = reader({ structured: "todo", doc: () => doc, reload: () => sheet?.scrollTo({ top: 0 }) });

  scrolls({
    reading: () => view.reading,
    structured: () => sheet,
    document: () => reading,
    when: () => todo.isTodo,
    onScroller: () => onScroller,
  });

  let spans = $derived(todo.items.map((item) => ({ from: item.line, to: item.to })));
  const port = viewport({
    span: () => clampedSpan(doc, spans[holding(spans, doc.line ?? 0)] ?? { to: doc.to ?? 0 }),
    epoch: () => answers.epoch,
  });
</script>

{#snippet done(of)}{of.done} of {of.total} done{/snippet}

{#if !todo.isTodo}
  <MarkdownPane doc={shown} {id} {active} {scrollRoot} />
{:else}
  <ArtifactPane
    {doc}
    structured="todo"
    label="Todos"
    mode={view.mode}
    onMode={(next) => (view.mode = next)}>
    {#snippet tag()}
      <ArtifactTag kind={doc.kind ?? ARTIFACT_KINDS.TODO} long />
    {/snippet}
    {#snippet body()}
      {#if view.reading}
        <div class="reading" bind:this={reading}>
          <h1 class="display-lg">{heading}</h1>
          <MarkdownPane doc={shown} {id} {active} {scrollRoot} bare />
        </div>
      {:else}
        <div
          class="sheet"
          bind:this={sheet}
          data-document-source={doc.source}
          use:links={doc.source}
          use:port={{ id, active, scrollRoot, request: answers.epoch }}>
          <section class="hero" data-overview>
            <CapsLabel>To do · {todo.items.length} {todo.items.length === 1 ? "item" : "items"}</CapsLabel>
            <h1 class="display-lg">{heading}</h1>
            <div class="tally">
              <span class="says" data-tally>{@render done(overall)}</span>
              {#if blocking > 0}
                <span class="blocking" data-blocking>{blocking} blocking</span>
              {/if}
            </div>
          </section>
          {#each groups as group (group.from)}
            <section class="group" data-section={group.name}>
              {#if group.name}
                <header class="group-head">
                  <CapsLabel>{group.name}</CapsLabel>
                  {#if group.rows.length > 0}
                    <span class="count" data-section-tally>{@render done(group)}</span>
                  {/if}
                </header>
              {/if}
              {#if group.prose}
                <div class="markdown prose-block">{@html group.prose}</div>
              {/if}
              {#if group.rows.length > 0}
                <ul class="items">
                  {#each group.rows as row (row.item.line)}
                    {@const item = row.item}
                    {@const save = saves.get(row.key)}
                    {@const named = pullRefs(item, tracked)}
                    <li
                      class="item"
                      class:next={row.key === next}
                      class:done={row.state === "done"}
                      class:skipped={row.state === "skipped"}
                      data-item={item.text}
                      data-state={row.state}
                      data-line={item.line}
                      data-line-end={item.to}>
                      <div class="row">
                        <Checkbox
                          checked={row.state === "done"}
                          disabled={row.state === "skipped"}
                          aria-label="Done: {item.text}"
                          onchange={() => mark(row, row.state === "done" ? "open" : "done")}>
                          <span class="text">{item.text}</span>
                        </Checkbox>
                        <span class="save" data-save={save?.state ?? ""}>{save ? (STATES[save.state] ?? "") : ""}</span>
                      </div>
                      {#if row.key === next || (row.state === "open" && item.blocks) || named.where || named.refs.length > 0}
                        <div class="chips">
                          {#if row.key === next}<Chip tone="running" selected data-next>Next</Chip>{/if}
                          {#if row.state === "open" && item.blocks}<Chip tone="waiting" data-blocks>Blocks {item.blocks}</Chip>{/if}
                          {#if named.where}<Chip data-where>Where {named.where}</Chip>{/if}
                          {#each named.refs as ref (ref.url)}
                            <a
                              class="pull"
                              href={ref.url}
                              title={ref.pull?.title ?? null}
                              data-pull={ref.pull?.state ?? ""}
                              onclick={(event) => (event.preventDefault(), openURL(ref.url))}>
                              {#if ref.pull}
                                <span
                                  class="pull-state"
                                  role="img"
                                  aria-label={STATE_WORDS[ref.pull.state]}
                                  title={STATE_WORDS[ref.pull.state]}
                                  style:color={ICON_COLOURS[ICONS[ref.pull.state]]}><Icon name={ICONS[ref.pull.state]} inherit /></span>
                              {/if}
                              {ref.pull?.repo ?? ref.repo}#{ref.number}
                            </a>
                          {/each}
                        </div>
                      {/if}
                      {#if item.run || item.check}
                        <dl class="fields">
                          {#if item.run}
                            <dt>Run</dt>
                            <dd data-run><CopyCode code={runCommand(item.run)} label="run command" /></dd>
                          {/if}
                          {#if item.check}
                            <dt>Check</dt>
                            <dd data-check>
                              {#if runCommand(item.check) !== item.check}
                                <CopyCode code={runCommand(item.check)} label="check" />
                              {:else}
                                {item.check}
                              {/if}
                            </dd>
                          {/if}
                        </dl>
                      {/if}
                      {#each item.prose as line}
                        <p class="prose">{line}</p>
                      {/each}
                      <div class="edit">
                        <input
                          class="note"
                          type="text"
                          placeholder="Add a note"
                          aria-label="Note for {item.text}"
                          value={row.note}
                          oninput={(event) => edit(row, { note: event.currentTarget.value }, true)}
                          onblur={() => answers.commit(row.key)}
                          onkeydown={(event) => {
                            if (event.key === "Enter") {
                              event.preventDefault();
                              answers.commit(row.key);
                            }
                          }} />
                        {#if row.state === "skipped"}
                          <Button variant="ghost" size="sm" data-reopen onclick={() => mark(row, "open")}>Reopen</Button>
                        {:else}
                          <Button variant="ghost" size="sm" data-skip onclick={() => mark(row, "skipped")}>Not needed</Button>
                        {/if}
                      </div>
                      {#if save?.message}
                        <div class="message" class:failed={save.state === "failed"} role="status">
                          <span>{save.message}</span>
                          <Button variant="outline" size="sm" data-retry onclick={() => answers.commit(row.key)}>Save again</Button>
                        </div>
                      {/if}
                    </li>
                  {/each}
                </ul>
              {/if}
            </section>
          {/each}
          {#if laid.detached.length > 0}
            <div class="detached" data-detached>
              <CapsLabel>Detached changes</CapsLabel>
              <p class="hint">These items changed or went away while you were editing. Your change is kept here to redo.</p>
              {#each laid.detached as draft (draft.key)}
                <div class="orphan" data-orphan={draft.text}>
                  <p class="kept">{written(draft)}</p>
                  <div class="actions">
                    <Button variant="outline" size="sm" onclick={() => copyText(written(draft))}>Copy</Button>
                    <Button variant="ghost" size="sm" onclick={() => answers.discard(draft.key)}>Discard</Button>
                  </div>
                </div>
              {/each}
            </div>
          {/if}
        </div>
      {/if}
    {/snippet}
    {#snippet bar()}
      <div class="send">
        <span class="send-button" title={blocked || null}>
          <Button variant={blocked ? "outline" : "primary"} size="sm" disabled={Boolean(blocked)} data-send onclick={send}>Tell the agent</Button>
        </span>
        <span class="why" class:failed={sent && !sent.ok} role="status">{blocked || sent?.message || ""}</span>
      </div>
    {/snippet}
    {#snippet counter()}
      <span class="counter">{@render done(overall)}</span>
    {/snippet}
  </ArtifactPane>
{/if}

<style>
  .sheet,
  .reading {
    flex: 1;
    position: relative;
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

  .hero > :global(.caps),
  .group-head {
    padding-left: var(--gutter);
  }

  .display-lg {
    margin: calc(14px * var(--ui-scale)) 0 calc(20px * var(--ui-scale));
    padding-left: var(--gutter);
    max-width: 26ch;
    font: var(--display-lg);
    letter-spacing: var(--display-tracking);
    color: var(--text-primary);
    overflow-wrap: anywhere;
  }

  .tally {
    display: flex;
    align-items: baseline;
    gap: calc(14px * var(--ui-scale));
    padding-left: var(--gutter);
  }

  .says {
    font: var(--machine-bold);
    font-size: calc(14px * var(--ui-scale));
    color: var(--text-primary);
  }

  .blocking {
    font: var(--machine-sm);
    color: var(--state-waiting);
  }

  .group {
    margin-top: calc(30px * var(--ui-scale));
  }

  .group-head {
    display: flex;
    align-items: baseline;
    gap: calc(12px * var(--ui-scale));
    margin-bottom: calc(10px * var(--ui-scale));
  }

  .count {
    font: var(--machine-sm);
    color: var(--text-muted);
  }

  .prose-block {
    padding-left: var(--gutter);
  }

  .items {
    display: flex;
    flex-direction: column;
    gap: calc(10px * var(--ui-scale));
    margin: 0;
    padding: 0 0 0 var(--gutter);
    list-style: none;
  }

  .item {
    display: flex;
    flex-direction: column;
    gap: calc(8px * var(--ui-scale));
    padding: calc(12px * var(--ui-scale)) calc(14px * var(--ui-scale));
    border: var(--border-width) solid var(--border-subtle);
  }

  .item.next {
    border-color: var(--artifact-todo);
    box-shadow: var(--shadow-offset) var(--artifact-todo);
  }

  .item:focus-within {
    border-color: var(--accent-action);
  }

  .row {
    display: flex;
    align-items: baseline;
    gap: calc(12px * var(--ui-scale));
  }

  .text {
    font: var(--machine-md);
    color: var(--text-primary);
    overflow-wrap: anywhere;
  }

  .done .text {
    color: var(--text-muted);
  }

  .skipped .text {
    color: var(--text-faint);
    text-decoration: line-through;
  }

  .save {
    margin-left: auto;
    flex: none;
    font: var(--machine-sm);
    color: var(--text-muted);
  }

  .save[data-save="saved"] {
    color: var(--state-success);
  }

  .save[data-save="unsaved"],
  .save[data-save="reloaded"],
  .save[data-save="saving"] {
    color: var(--state-waiting);
  }

  .save[data-save="failed"],
  .message.failed {
    color: var(--state-failed);
  }

  .chips {
    display: flex;
    flex-wrap: wrap;
    gap: calc(6px * var(--ui-scale));
  }

  .pull {
    display: inline-flex;
    align-items: center;
    gap: calc(6px * var(--ui-scale));
    padding: 2px calc(7px * var(--ui-scale));
    border: 1px solid var(--border-subtle);
    background: var(--surface-chrome);
    font: var(--machine-sm);
    font-size: calc(11px * var(--ui-scale));
    color: var(--text-secondary);
    text-decoration: none;
  }

  .pull:hover {
    background: var(--wash-selected);
  }

  .pull-state {
    display: inline-flex;
  }

  .pull-state :global(.icon) {
    width: calc(13px * var(--ui-scale));
    height: calc(13px * var(--ui-scale));
  }

  .fields {
    display: grid;
    grid-template-columns: max-content minmax(0, 1fr);
    align-items: baseline;
    gap: calc(6px * var(--ui-scale)) calc(14px * var(--ui-scale));
    margin: 0;
    font: var(--machine-md);
  }

  dt {
    font: var(--machine-sm);
    color: var(--text-muted);
  }

  dd {
    min-width: 0;
    margin: 0;
    color: var(--text-secondary);
    overflow-wrap: anywhere;
  }

  .prose {
    margin: 0;
    font: var(--machine-sm);
    color: var(--text-secondary);
    overflow-wrap: anywhere;
  }

  .edit {
    display: flex;
    align-items: center;
    gap: calc(8px * var(--ui-scale));
  }

  .note {
    flex: 1;
    min-width: 0;
    padding: calc(5px * var(--ui-scale)) calc(8px * var(--ui-scale));
    border: var(--border-width) solid var(--border-subtle);
    background: var(--surface-terminal);
    color: var(--text-primary);
    font: var(--machine-sm);
  }

  .note:focus {
    outline: none;
    border-color: var(--accent-action);
  }

  .message {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: calc(8px * var(--ui-scale));
    font: var(--machine-sm);
    color: var(--state-waiting);
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

  .actions {
    display: flex;
    gap: calc(8px * var(--ui-scale));
  }

  :global(html[data-narrow]) .hero {
    padding-top: calc(6px * var(--ui-scale));
  }

  :global(html[data-narrow]) .display-lg {
    font: var(--display-md);
  }

  :global(html[data-narrow]) .items,
  :global(html[data-narrow]) .tally,
  :global(html[data-narrow]) .prose-block,
  :global(html[data-narrow]) .detached {
    padding-left: 0;
  }
</style>
