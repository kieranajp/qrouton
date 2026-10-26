<script>
  import { ARTIFACT_KINDS, AUDIT_HEADINGS } from "../bridge/generated.js";
  import ArtifactTag from "../core/ArtifactTag.svelte";
  import Button from "../core/Button.svelte";
  import CapsLabel from "../core/CapsLabel.svelte";
  import { copyText } from "../wails.js";
  import ArtifactPane from "./ArtifactPane.svelte";
  import FindingCard from "./FindingCard.svelte";
  import { links, viewport } from "./actions.js";
  import { answerable } from "./answerable.svelte.js";
  import { CHOICES, IMPACTS, TELLS, TRIAGED, byImpact, parseAudit, reapply, spliceTriage } from "./audit.js";
  import { clampedSpan, holding, partition } from "./deck.js";
  import MarkdownPane from "./MarkdownPane.svelte";
  import { render } from "./markdown.js";
  import { reader, scrolls } from "./reader.svelte.js";
  import { sendAuditTriage } from "./spec-calls.js";
  import "./markdown.css";

  /** @type {{doc: {text: string, hash?: string, format: string, source: string, path?: string, kind?: string, tells?: string[], line?: number, to?: number, viewportEpoch?: number}, id: string, active?: boolean, scrollRoot?: HTMLElement, onScroller?: (element: HTMLElement | null) => void}} */
  let { doc, id, active = false, scrollRoot, onScroller } = $props();

  /** @typedef {{id: string, choice: string, note: string}} Draft */

  const STEPS = [
    "",
    "No tie",
    "No reason that holds",
    "The guarded case cannot occur",
    "The copies disagree",
    "Nothing shipped reaches it",
    "The test never runs",
    "The test cannot fail",
    "Shipped code changes under test",
    "A harmless change breaks it",
    "Another test covers it",
  ];

  const answers = answerable({
    id: () => id,
    doc: () => doc,
    splice,
    reloaded: "The audit changed on disk, so it reloaded. Your triage is kept here: choose again or press ⌘↵ to save it.",
  });
  const { pending, saves } = answers;

  /** @param {string} text @param {string} key @param {Draft} draft @returns {string | null} */
  function splice(text, key, draft) {
    const finding = audit.findings.find((finding) => finding.id === key);
    return finding ? spliceTriage(text, finding, draft) : null;
  }

  let source = $derived(answers.source);
  let shown = $derived({ ...doc, text: source.text, viewportEpoch: answers.epoch });
  let rendered = $derived(render(source.text));
  let audit = $derived(parseAudit(source.text, doc.tells ?? null));
  let laid = $derived(reapply(audit, pending));
  let heading = $derived(rendered.title || (doc.source ? doc.source.split("/").pop() : ""));
  let parts = $derived(partition(rendered.body, [...audit.sections, ...audit.tells.filter((tell) => tell.from)]));
  let summary = $derived(audit.sections.findIndex((section) => section.name.toLowerCase() === AUDIT_HEADINGS.SUMMARY.toLowerCase()));

  /** @type {Map<string, {choice: string, note: string}>} */
  let drafts = $derived(
    new Map(laid.findings.map(({ finding, draft }) => [finding.id, draft ?? { choice: finding.triage?.choice ?? "", note: finding.triage?.note ?? "" }])),
  );
  /** @param {string} findingID */
  const draftOf = (findingID) => drafts.get(findingID) ?? { choice: "", note: "" };
  let sorted = $derived(byImpact(audit.findings));
  let triaged = $derived([...drafts.values()].filter((draft) => draft.choice || draft.note.trim()).length);

  /** @param {{choice: string, note: string}} draft */
  const triageOf = (draft) => TRIAGED[draft.choice] ?? (draft.note.trim() ? "Note only" : "Untriaged");

  /** @param {string[]} keys @param {string[]} order */
  function tallied(keys, order) {
    const counts = new Map(order.map((key) => [key, 0]));
    for (const key of keys) counts.set(key, (counts.get(key) ?? 0) + 1);
    return [...counts].map(([name, count]) => ({ name, count }));
  }

  let tallies = $derived([
    { label: "By tell", rows: audit.tells.map((tell) => ({ name: tell.name, count: tell.count })) },
    {
      label: "By verdict",
      rows: tallied(
        audit.findings.map((finding) => (finding.step ? `Step ${finding.step} · ${STEPS[finding.step]}` : "No step named")),
        [],
      ).sort((a, b) => a.name.localeCompare(b.name)),
    },
    {
      label: "By impact",
      rows: tallied(
        audit.findings.map((finding) => (IMPACTS.includes(finding.fields.impact) ? finding.fields.impact : "unrated")),
        [...IMPACTS, "unrated"],
      ),
    },
    {
      label: "By triage",
      rows: tallied([...drafts.values()].map(triageOf), [...CHOICES.map((choice) => TRIAGED[choice]), "Note only", "Untriaged"]),
    },
  ]);

  /** @param {string} findingID @param {Partial<{choice: string, note: string}>} change */
  function edit(findingID, change, typed = false) {
    sent = null;
    const draft = { id: findingID, ...draftOf(findingID), ...change };
    if (typed) answers.type(findingID, draft);
    else answers.put(findingID, draft);
  }

  /** @param {string} findingID @param {string} choice */
  function choose(findingID, choice) {
    edit(findingID, { choice: draftOf(findingID).choice === choice ? "" : choice });
    answers.commit(findingID);
  }

  let blocked = $derived.by(() => {
    const waiting = answers.waiting;
    if (waiting === "saving") return "Wait for the save to finish.";
    if (waiting === "failed") return "A triage did not save. Save it again first.";
    if (laid.detached.length > 0) return "Copy or discard the detached triage first.";
    if (waiting === "typed") return "Save your typed note first: click outside the field or press ⌘↵.";
    if (triaged === 0) return "Triage at least one finding first.";
    return "";
  });
  let sent = $state(/** @type {{ok: boolean, message: string} | null} */ (null));

  async function send() {
    if (blocked) return;
    const typed = await sendAuditTriage(id);
    sent = typed.ok
      ? { ok: true, message: "Typed into the conversation." }
      : { ok: false, message: `Could not type into the conversation: ${typed.error?.message ?? typed.error}` };
  }

  /** @type {HTMLElement | undefined} */
  let sheet = $state();
  /** @type {HTMLElement | undefined} */
  let reading = $state();

  const view = reader({ structured: "audit", doc: () => doc, reload: () => sheet?.scrollTo({ top: 0 }) });

  scrolls({
    reading: () => view.reading,
    structured: () => sheet,
    document: () => reading,
    when: () => audit.isAudit,
    onScroller: () => onScroller,
  });

  const port = viewport({
    span: () => clampedSpan(doc, audit.findings[holding(audit.findings, doc.line ?? 0)] ?? { to: doc.to ?? 0 }),
    epoch: () => answers.epoch,
  });
</script>

{#if !audit.isAudit}
  <MarkdownPane doc={shown} {id} {active} {scrollRoot} />
{:else}
  <ArtifactPane
    {doc}
    structured="audit"
    label="Findings"
    mode={view.mode}
    onMode={(next) => (view.mode = next)}>
    {#snippet tag()}
      <ArtifactTag kind={doc.kind ?? ARTIFACT_KINDS.AUDIT} long />
    {/snippet}
    {#snippet body()}
      {#if view.reading}
        <div class="reading" bind:this={reading}>
          <h1 class="display-lg">{audit.title || heading}</h1>
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
            <CapsLabel>Audit · {audit.findings.length} {audit.findings.length === 1 ? "finding" : "findings"}</CapsLabel>
            <h1 class="display-lg">{audit.title || heading}</h1>
            <div class="markdown lead">{@html parts.preamble}</div>
            {#if summary >= 0}
              <div class="markdown lifted">{@html parts.sections[summary].opening}</div>
              <div class="markdown lead">{@html parts.sections[summary].body}</div>
            {/if}
            <div class="tally" data-tally>
              <span class="says">{audit.findings.length === 0 ? "No findings" : `${triaged} of ${audit.findings.length} triaged`}</span>
            </div>
            {#if audit.tells.length < TELLS.length}
              <p class="ran" data-tells-run>Ran {audit.tells.length} of {TELLS.length} tells: {audit.tells.map((tell) => tell.name).join(", ")}</p>
            {/if}
            {#if audit.findings.length > 0}
              <div class="tallies">
                {#each tallies as tally}
                  <div class="group" data-group={tally.label}>
                    <CapsLabel tone="dim">{tally.label}</CapsLabel>
                    <ul>
                      {#each tally.rows as row}
                        <li class:none={row.count === 0}>
                          <span class="name">{row.name}</span>
                          <span class="bar"><span style:width="{(row.count / audit.findings.length) * 100}%"></span></span>
                          <span class="count">{row.count}</span>
                        </li>
                      {/each}
                    </ul>
                  </div>
                {/each}
              </div>
            {/if}
          </section>
          {#if sorted.length > 0}
            <CapsLabel data-list>Findings, highest impact first</CapsLabel>
            <div class="cards">
              {#each sorted as finding (finding.id)}
                <FindingCard
                  {finding}
                  draft={draftOf(finding.id)}
                  state={saves.get(finding.id)?.state}
                  message={saves.get(finding.id)?.message}
                  onChoose={(choice) => choose(finding.id, choice)}
                  onNote={(note) => edit(finding.id, { note }, true)}
                  onCommit={() => answers.commit(finding.id)} />
              {/each}
            </div>
          {/if}
          {#if laid.detached.length > 0}
            <div class="detached" data-detached>
              <CapsLabel>Detached triage</CapsLabel>
              <p class="hint">These findings were removed while you were triaging. Your text is kept here.</p>
              {#each laid.detached as draft (draft.id)}
                <div class="orphan" data-orphan={draft.id}>
                  <span class="was">{draft.id}</span>
                  <p class="kept">{[draft.choice, draft.note].filter(Boolean).join(" — ")}</p>
                  <div class="actions">
                    <Button variant="outline" size="sm" onclick={() => copyText([draft.choice, draft.note].filter(Boolean).join("\n\n"))}>Copy</Button>
                    <Button variant="ghost" size="sm" onclick={() => answers.discard(draft.id)}>Discard</Button>
                  </div>
                </div>
              {/each}
            </div>
          {/if}
          {#each audit.sections as section, index}
            {#if index !== summary}
              <section class="pinned" data-section={section.name}>
                <CapsLabel>{section.name}</CapsLabel>
                <div class="markdown lifted">{@html parts.sections[index].opening}</div>
                <div class="markdown">{@html parts.sections[index].body}</div>
              </section>
            {/if}
          {/each}
        </div>
      {/if}
    {/snippet}
    {#snippet bar()}
      {#if audit.findings.length > 0}
        <div class="send">
          <span class="send-button" title={blocked || null}>
            <Button
              variant={triaged === audit.findings.length && !blocked ? "primary" : "outline"}
              size="sm"
              disabled={Boolean(blocked)}
              data-send
              onclick={send}>Type 'plan the accepted findings' into the conversation</Button>
          </span>
          <span class="why" class:failed={sent && !sent.ok} role="status">{blocked || sent?.message || ""}</span>
        </div>
      {/if}
    {/snippet}
    {#snippet counter()}
      <span class="counter">{triaged} of {audit.findings.length} triaged</span>
    {/snippet}
  </ArtifactPane>
{/if}

<style>
  .sheet,
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

  .hero > :global(.caps),
  .sheet > :global(.caps[data-list]),
  .pinned > :global(.caps) {
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

  .lead :global(p) {
    font: var(--machine-lg);
    font-size: calc(15px * var(--ui-scale));
    line-height: 1.7;
    color: var(--text-secondary);
    max-width: 68ch;
  }

  .tally {
    margin-top: calc(26px * var(--ui-scale));
    padding-left: var(--gutter);
  }

  .tally .says {
    font: var(--machine-bold);
    font-size: calc(14px * var(--ui-scale));
    color: var(--text-primary);
  }

  .ran {
    margin: calc(8px * var(--ui-scale)) 0 0;
    padding-left: var(--gutter);
    font: var(--machine-sm);
    color: var(--text-muted);
  }

  .tallies {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(calc(220px * var(--ui-scale)), 1fr));
    gap: calc(18px * var(--ui-scale));
    margin-top: calc(18px * var(--ui-scale));
    padding-left: var(--gutter);
  }

  .group ul {
    list-style: none;
    margin: calc(8px * var(--ui-scale)) 0 0;
    padding: 0;
    border: var(--border-width) solid var(--border-subtle);
    box-shadow: var(--shadow-offset) var(--border-subtle);
  }

  .group li {
    display: grid;
    grid-template-columns: minmax(0, 1fr) calc(48px * var(--ui-scale)) 3ch;
    align-items: center;
    gap: calc(10px * var(--ui-scale));
    padding: calc(8px * var(--ui-scale)) calc(12px * var(--ui-scale));
    font: var(--machine-sm);
    color: var(--text-secondary);
  }

  .group li + li {
    border-top: var(--border-width) solid var(--border-subtle);
  }

  .group li.none {
    color: var(--text-faint);
  }

  .group .name {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .bar {
    height: calc(4px * var(--ui-scale));
    background: var(--border-subtle);
  }

  .bar > span {
    display: block;
    height: 100%;
    background: var(--artifact-audit);
  }

  .count {
    font: var(--machine-bold);
    text-align: right;
    color: var(--text-primary);
  }

  .none .count {
    color: var(--text-faint);
  }

  .sheet > :global(.caps[data-list]) {
    display: block;
    margin: calc(30px * var(--ui-scale)) 0 calc(10px * var(--ui-scale));
  }

  .cards {
    display: flex;
    flex-direction: column;
    gap: calc(22px * var(--ui-scale));
    padding-left: var(--gutter);
  }

  .pinned {
    margin-top: calc(30px * var(--ui-scale));
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

  :global(html[data-narrow]) .hero {
    padding-top: calc(6px * var(--ui-scale));
  }

  :global(html[data-narrow]) .display-lg {
    font: var(--display-md);
  }

  :global(html[data-narrow]) .cards,
  :global(html[data-narrow]) .tally,
  :global(html[data-narrow]) .ran,
  :global(html[data-narrow]) .tallies,
  :global(html[data-narrow]) .detached {
    padding-left: 0;
  }
</style>
