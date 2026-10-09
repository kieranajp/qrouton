import assert from "node:assert/strict";
import { test } from "node:test";
import { parseSpec } from "./spec.js";

const doc = (...lines) => lines.join("\n");

const OPEN = doc(
  "---",
  "kind: spec",
  "---",
  "",
  "# Retry timeouts",
  "",
  "## End state",
  "",
  "Callers can bound a retry.",
  "",
  "## Open questions",
  "",
  "### Q1 — Per attempt or overall?",
  "",
  "It changes the signature.",
  "",
  "- A. Per attempt",
  "- B. Overall **(recommended: callers care about their own wait)**",
  "",
  "Answer: B",
  "",
  "### Q2: Where does the deadline come from?",
  "",
  "- A. A context",
  "- B. A duration",
  "",
  "Answer: A, because the callers have one",
  "",
  "### Q3 - What happens on expiry?",
  "",
  "- A. Return the last error",
  "- B. Return a timeout error",
  "",
  "Answer:",
  "",
  "### Q4 — Free text?",
  "",
  "- A. Yes",
  "- B. No",
  "",
  "Answer:",
  "",
  "Neither: log it and carry on.",
  "",
  "## Decisions",
  "",
  "**Retries stay bounded.** The attempt count holds.",
  "Rejected: unbounded retries.",
  "",
  "### Q0 — Which package owns it?",
  "",
  "Chosen: account.",
  "Rejected: A. shared-lib: reference only.",
  "",
  "## Risks",
  "",
  "None.",
  "",
);

const ZERO = doc(
  "# Audit log",
  "",
  "## Open questions",
  "",
  "None.",
  "",
  "## Decisions",
  "",
  "**The service writes entries.** The store stays plain.",
  "",
  "**Append before put.** A failed append stops the change.",
  "",
);

const FREEFORM = doc("# Old spec", "", "## Approach", "", "Do the thing.", "", "## Decisions", "", "- use X", "");

test("questions under Open questions are read with their options and answers", () => {
  const spec = parseSpec(OPEN);
  assert.equal(spec.isSpec, true);
  assert.equal(spec.title, "Retry timeouts");
  assert.deepEqual(spec.open.map((q) => [q.id, q.heading]), [
    ["Q1", "Per attempt or overall?"],
    ["Q2", "Where does the deadline come from?"],
    ["Q3", "What happens on expiry?"],
    ["Q4", "Free text?"],
  ]);
  assert.equal(spec.open[0].context, "It changes the signature.");
  assert.equal(spec.answered, 3);
});

test("a recommended option keeps its reason and loses the marker", () => {
  const [, recommended] = parseSpec(OPEN).open[0].options;
  assert.deepEqual(recommended, {
    letter: "B",
    text: "Overall",
    recommended: true,
    reason: "callers care about their own wait",
  });
});

test("a bare letter, a letter with a note, free text and nothing are told apart", () => {
  const [one, two, three, four] = parseSpec(OPEN).open;
  assert.deepEqual([one.answer.letter, one.answer.note], ["B", ""]);
  assert.deepEqual([two.answer.letter, two.answer.note], ["A", "because the callers have one"]);
  assert.equal(three.answer.raw, "");
  assert.deepEqual([four.answer.letter, four.answer.note], ["", "Neither: log it and carry on."]);
});

test("an answer block spans its own lines only", () => {
  const [one, , three, four] = parseSpec(OPEN).open;
  assert.deepEqual([one.answer.from, one.answer.to], [20, 20]);
  assert.deepEqual([three.answer.from, three.answer.to], [34, 34]);
  assert.deepEqual([four.answer.from, four.answer.to], [41, 43]);
});

test("decisions come from bold leads and resolved questions", () => {
  const spec = parseSpec(OPEN);
  assert.deepEqual(
    spec.decisions.map((d) => [d.kind, d.id, d.label]),
    [
      ["decision", "", "Retries stay bounded"],
      ["question", "Q0", "Which package owns it?"],
    ],
  );
  assert.deepEqual(spec.sections.map((s) => s.name), ["End state", "Risks"]);
});

test("a spec with no open questions is still a spec", () => {
  const spec = parseSpec(ZERO);
  assert.equal(spec.isSpec, true);
  assert.deepEqual(spec.open, []);
  assert.equal(spec.answered, 0);
  assert.deepEqual(spec.decisions.map((d) => d.label), ["The service writes entries", "Append before put"]);
});

test("a freeform spec is not one", () => {
  assert.equal(parseSpec(FREEFORM).isSpec, false);
});

test("a decision runs up to the next one", () => {
  const [first, second] = parseSpec(ZERO).decisions;
  assert.deepEqual([first.from, first.to], [9, 10]);
  assert.deepEqual([second.from, second.to], [11, 12]);
});

test("the spec this pane was built from reads as fully resolved", () => {
  const spec = parseSpec(INTEGRAL);
  assert.equal(spec.isSpec, true);
  assert.deepEqual(spec.open, []);
  const questions = spec.decisions.filter((d) => d.kind === "question");
  assert.deepEqual(questions.map((q) => q.id), ["Q1", "Q2", "Q3", "Q4", "Q5"]);
  assert.equal(spec.decisions.filter((d) => d.kind === "decision").length, 13);
});

const INTEGRAL = doc(
  "---",
  "kind: spec",
  "title: Spec by default, answered in place",
  "research: ../research/R1-2026-10-08-artifact-panes-and-spec-routing.md",
  "status: answered",
  "---",
  "",
  "# Spec by default, answered in place",
  "",
  "## End state",
  "",
  "Between Research and Plan, the orchestrator writes a spec unless the work is small or every choice is already explicit. The spec puts each open question in the file, with options and a recommendation. A spec under `specs/` opens in a SpecPane. Its overview shows \"n of m answered\", and every open question sits on one screen as a card. The user types answers into the cards, and the pane saves them into the file without losing anyone's edits. A Send button tells the agent the answers are in. The agent then folds each answered question into `## Decisions`, keeping what was asked, what was chosen and what was rejected. The user still sees three stages: Research, Plan, Implement.",
  "",
  "Code paths below are relative to `src/qrouton`.",
  "",
  "## Scope",
  "",
  "**In:**",
  "",
  "- A spec document shape, written into the spec skill as a reference file.",
  "- Orchestrator routing that makes the spec the default, with a high bar for asking questions.",
  "- Routing evals for the spec route, including a spec with no open questions.",
  "- SpecPane, its `spec.js` parser and tests, registered for `ARTIFACT_KINDS.SPEC`.",
  "- A Go binding that writes answers back to the spec file, with a stale check.",
  "- A Send button that types one fixed line into the conversation.",
  "",
  "**Out:**",
  "",
  "- Migrating the ~60 existing freeform specs. They keep rendering in MarkdownPane.",
  "- Answer editing for plans, research or notes.",
  "- Reading or parsing the runner's replies. qrouton never learns whether the agent acted on Send.",
  "- A file watcher. The one-second stat poll stays (`internal/desktop/documents.go:148-149`).",
  "",
  "## Decisions",
  "",
  "**Answers live in the spec file.** No sidecar and no hidden state. The agent writes every question into the document, even when the user answers in chat. Answering in chat still works; the agent then records the answer in the file.",
  "Rejected: questions asked only in chat with outcomes written afterwards. The sensible-milestones spec did this and lost what was asked and what was recommended (research, \"The resolved form\").",
  "",
  "**Answered questions keep the record.** A resolved question keeps its question text, the chosen option, and each rejected option with its reason.",
  "Rejected: bare decision lines, which drop what the choice beat.",
  "",
  "**Lifecycle is open → answered → resolved.** The pane only moves a question from open to answered, by writing its answer block. The agent moves an answered question into `## Decisions`. The pane never edits Decisions.",
  "Rejected: a pane that folds answers itself. That would make qrouton decide what an answer means, and free text needs judgement.",
  "",
  "**Free text is a full answer.** A letter, a letter with a note, and text that names no letter are all answers. The parser reads a leading option letter if there is one and keeps the rest as a note. Any non-empty answer counts as answered. In app-review, 11 of 29 answers were not a bare letter (research, app-review counts).",
  "Rejected: forcing a letter, which would have refused a third of app-review's answers.",
  "",
  "**SpecPane builds on PlanPane's reader deck.** Screen 0 is an overview with \"n of m answered\" and the list of decisions. The next pip holds every open question, as cards on one screen. After it, each decision gets its own pip, as PlanPane gives each phase one (`internal/desktop/frontend/src/lib/panes/PlanPane.svelte:116-125`). That covers both resolved `### Q<n>` blocks and bold-led decisions. Other H2 sections follow as reader screens. Only answer blocks are editable. The Document toggle shows the full text through `MarkdownPane bare`. A spec that doesn't follow the format falls back to MarkdownPane, as PlanPane does when it finds no H2 (`PlanPane.svelte:140-141`).",
  "Rejected:",
  "",
  "- All decisions on one screen. A long Decisions section reads as a wall, and a single decision can't be pointed at.",
  "- Migrating old specs into the new format. Most hold only the resolved form, so there is nothing to answer.",
  "",
  "**The staleness token is a SHA-256 of the bytes Go read.** Go adds a `hash` field to the document payload (`internal/desktop/documents.go:19-33`), on the first fetch and on every push. The pane sends that hash back with its save. Go re-reads the file, hashes it, and refuses with a sentinel `ErrDocumentChanged` if the hash differs. Otherwise it replaces the file through `atomicfile.Replace` (`internal/atomicfile/atomicfile.go:16`), keeping the file's existing mode. When the pane gets `ErrDocumentChanged`, it takes the fresh text, re-parses, and lays its unsaved answers back over the matching questions.",
  "Rejected:",
  "",
  "- Size and mtime as the token. The poll already uses them and misses an edit that keeps both (research, \"Change detection\").",
  "- The existing `Revision` counter. It is a per-push sequence for image galleries, not a property of the file, so it cannot detect a write the poll has not seen yet (`internal/desktop/frontend/src/lib/DocumentPane.svelte:34-35`).",
  "- A diff-and-merge on save. It is more code for a case the pane already handles by reloading.",
  "",
  "**The pane splices; Go compares and swaps.** `spec.js` rewrites the answer block in the text it rendered and sends the whole new text. Go knows nothing about the question format. The binding accepts only an open document window whose kind is `SPEC`, so it cannot write arbitrary files.",
  "Rejected: Go splicing answers by question id. That needs a second Markdown parser in Go, kept in step with `spec.js`.",
  "",
  "**Unsaved answers are keyed by question id and question text.** After a reload, a draft goes back onto the question with the same id and heading. If the agent renamed or removed that question, the pane shows the draft as detached, with its text, and does not drop it.",
  "Rejected: keying by id alone. The agent can renumber, and a draft would land on the wrong question.",
  "",
  "**The Send button types a fixed line plus Enter.** A bound Go method writes the line through the conversation's existing write path (`internal/desktop/session.go:159-164`), so agent state sees it as user input. The line lives in `internal/desktop/strings.go` and names the spec's session-relative path. Nothing in it depends on the model's output. Send is disabled while answers are unsaved or the save failed. The label says what it does, for example \"Type 'answers are in' into the conversation\".",
  "Accepted risk: a half-typed draft in the runner's input box is submitted with the line, because qrouton cannot see that buffer.",
  "Rejected: the page building the line and calling `Term.Write`. The copy would move into Svelte, against the `strings.go` rule (`AGENTS.md:56`). Also rejected: a line with no path, which leaves the agent guessing when a session has several specs.",
  "",
  "**RPI stays.** The orchestrator presents spec questions as \"design questions\" within Plan, never as a fourth stage.",
  "Rejected: a visible Spec stage, which adds a step to every piece of work the user tracks.",
  "",
  "**Spec by default, with a high bar for questions.** The orchestrator skips the spec only when the work is small or every choice is already explicit. A question goes in only when a wrong guess means rework. A spec with zero open questions is valid. Answering questions does not authorise the Plan stage; the user still asks for it.",
  "Rejected: today's rule, which routes to a spec only when \"a material choice\" stays open and never defines material (`prompts/orchestrator.md:21-29`).",
  "",
  "**SpecPane is a rich, interactive answering surface.** Each open question is a card. It shows the question, its context, and each option as a large selectable choice. The recommended option is visibly marked, with its reason. One click picks an option and saves. Each card also has a free-text field for a note, or for an answer that names no option. The overview shows \"n of m answered\" and moves the reader to the next open question. Keyboard shortcuts pick options and move between cards. Answered cards show the chosen option and stay editable until the agent folds them. When every question is answered, Send becomes the obvious next step.",
  "Rejected: a form that mirrors the Markdown, where the user types after `Answer:` in a text box. It is no better than nvim.",
  "",
  "**The pane ignores `status:`.** Counts come from parsing the questions. Free-prose status goes stale (research, app-review still \"awaiting decisions\" after 29 answers).",
  "Rejected: driving the overview from frontmatter status.",
  "",
  "### Q1 — Which question format should specs use?",
  "",
  "Answer (in chat): \"No, richer UI than this even. Currently I cannot answer on the spec in the sidebar: I had to nvim the .md file in the example I gave you. This has to be inline and has to be a rich UI that encourages interaction.\"",
  "",
  "Chosen: SpecPane is where the user answers, and it must invite answering. The Markdown is the storage format the pane reads and writes. A plain editor stays a fallback, not the intended path. The storage format is the hybrid: a `### Q1 — …` heading, a context paragraph, lettered options with `**(recommended: reason)**`, and one `Answer:` block running to the next heading. The heading gives the pane a stable id and a boundary (`internal/desktop/frontend/src/lib/panes/sections.js:39`). The `Answer:` block holds a letter, a letter with a note, or free text, which matches how app-review's 29 answers were actually written.",
  "",
  "Rejected:",
  "",
  "- A. GFM task list with `- [ ] Other:` and `> Note:`. A tick plus a note spans two places in the file, and the pane would have to keep both in step. The checkboxes buy nothing once the pane draws the options.",
  "- B. App-review's bold `**1. …?**` questions under topic H3s. The pane would pair questions and answers by regex rather than by heading, and the numbers carry no stable id.",
  "- Answering in a plain editor as the main path. App-review shows the cost: the user had to open the file in nvim to answer.",
  "",
  "### Q2 — What shape does a resolved question take in `## Decisions`?",
  "",
  "The agreed record is the question, the chosen option, and the rejected options with reasons. The parser needs a fixed shape so it can count only open questions and give each decision its own pip.",
  "",
  "Answer: A",
  "",
  "Chosen: move the whole `### Q<n> — …` block under `## Decisions`, keeping its id and heading. Replace the options with a `Chosen:` line and one `Rejected:` line per option, each with its reason. Keep any note. This block follows that shape.",
  "",
  "Rejected:",
  "",
  "- B. Rewrite as `### D1 — <decision>` with an `Asked as Q1:` line. It matches older D-id specs, but numbering restarts and the question becomes secondary.",
  "- C. Leave the question in place with a `Settled:` line, as app-review did. App-review's Q29 settled Q10 while Q10 still read `Answer: A`, so the page contradicted itself.",
  "",
  "### Q3 — How should routing evals tell spec routing from plan routing?",
  "",
  "`first_delegation` matches lead names, and both skills delegate to `qrouton-planning-lead` (`internal/evalharness/grade.go:303-321`, `prompts/skills/qrouton-spec/SKILL.md:12-13`).",
  "",
  "Answer: A",
  "",
  "Chosen: assert on the artifact. A spec case uses `artifact_exists thoughts/shared/specs/*.md` with `artifact_absent thoughts/shared/plans/*.md`, and the skip case uses the reverse. Both check kinds exist already (`grade.go:75-118`), so the harness needs no change.",
  "",
  "Rejected:",
  "",
  "- B. A `qrouton-spec-lead`. It is a new agent definition to keep in step with the planning lead, added only for grading.",
  "- C. A `skill_invoked` check kind. It needs per-runner transcript parsing for skill calls, which the harness does not do.",
  "",
  "### Q4 — How does a document become a SpecPane?",
  "",
  "Kind comes from the path alone (`internal/status/status.go:420-425`). Existing specs split their frontmatter between `kind:`, `type:` and nothing, and the desktop never reads it.",
  "",
  "Answer: A, yep, similar to Plan. Also, similar to Plan, decisions can go to new pages in there. Keep the open questions on their own pip.",
  "",
  "Chosen: path kind `SPEC`, then `spec.js` checks the format. A spec gets SpecPane when it has an `## Open questions` H2 with `### Q<n> — ` headings, or a `## Decisions` H2 holding resolved Q blocks. Anything else falls back to MarkdownPane, as PlanPane falls back. The page layout follows from the note: the open questions share one pip, and each decision gets its own.",
  "",
  "Rejected:",
  "",
  "- B. A frontmatter key such as `format: questions`. Go would have to start reading frontmatter for kind, and a missing key silently gives the old pane.",
  "- C. Path kind plus the frontmatter key. Two things to keep right for one outcome.",
  "",
  "### Q5 — What does SpecPane show when no question is open?",
  "",
  "Every spec ends here once its answers are folded in, and a zero-question spec starts here.",
  "",
  "Answer: \"I think that is also answered by 4?\"",
  "",
  "Chosen: yes, by Q4's layout. The spec stays in SpecPane. The overview reads \"No open questions\" and lists the decisions. The open-questions pip is dropped, each decision keeps its own pip, and Send is hidden. The pane does not change when the last answer is folded in.",
  "",
  "Rejected: B. Falling back to MarkdownPane, which swaps the pane under the user at that moment.",
  "",
  "## Phases",
  "",
  "Each phase ships on its own and passes `make check`.",
  "",
  "1. **Format and prompts.** Add `prompts/skills/qrouton-spec/references/spec-shape.md` with the chosen format. Rewrite the orchestrator's spec and plan rows to make the spec the default, with the skip conditions, the question bar and the \"design questions within Plan\" wording. Add eval scenarios and fixtures: research with an open choice routes to a spec; small, explicit work routes straight to a plan; a large, settled piece of work yields a zero-question spec.",
  "2. **Read-only SpecPane.** Add `panes/spec.js` and its Vitest file, `SpecPane.svelte`, and the `KINDS` entry for `ARTIFACT_KINDS.SPEC` (`internal/desktop/frontend/src/lib/panes/index.js:20-23`). Add a Playwright spec with a fixture covering the overview, the open-questions pip, one pip per decision, the zero-question case and the MarkdownPane fallback.",
  "3. **Answer writes.** Add `hash` to the document payload. Add the bound compare-and-swap method, `ErrDocumentChanged` in `internal/desktop/errors.go`, and Go tests against `t.TempDir()` for a clean write, a stale write, a non-spec window and mode preservation. Make the cards editable, with reload-and-reapply on a stale save.",
  "4. **Send.** Add the bound Send method and its line in `internal/desktop/strings.go`. Add the button with its plain label and its disabled states. Add an `AGENTS.md` invariant beside the focus rule: qrouton writes into the conversation only when the user presses a control that says so, and only a fixed line it never adapts to the model's output; Send is that one case and sets no precedent for reading replies or choosing when to speak. Go test the bytes written; Playwright test the bridge call.",
  "",
  "## Done when",
  "",
  "- `make check` passes after each phase.",
  "- In a real session, a spec written by the agent renders in SpecPane.",
  "- The user answers in the pane and the answers land in the file.",
  "- The agent edits the spec while answers are unsaved; the save is refused, the pane reloads, and no answer is lost.",
  "- After Send, the agent folds every answered question into `## Decisions` in the agreed shape, and the overview reads \"No open questions\".",
  "",
  "## Risks and dependencies",
  "",
  "- **Check-then-replace gap.** Go hashes, then renames. An agent write landing between the two is lost. The gap is milliseconds, and agents don't take qrouton's `flock`, so `WithLock` would not close it. Accepted.",
  "- **The agent's own stale edit.** If the pane saves while the agent holds an older read, the agent's next edit may fail or overwrite answers, depending on the runner's edit tool. The spec-shape reference tells the agent to re-read the spec before folding.",
  "- **Missed pushes.** The poll can miss an edit that keeps size and mtime (`internal/desktop/documents.go:188`). The hash check catches it at save time. The pane may still show old text until then.",
  "- **Phase 1 changes routing for every session.** A loose question bar would put trivial work through a spec. The skip-case eval guards it.",
  "",
);
