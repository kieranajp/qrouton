import { readFileSync } from "node:fs";
import assert from "node:assert/strict";
import { test } from "node:test";
import { parseTodo, pullRefs, reapply, runCommand, setState, spliceDraft, todoKey } from "./todo.js";

const doc = (...lines) => lines.join("\n");

const TODO = doc(
  "---",
  "kind: todo",
  "---",
  "",
  "# Rollout",
  "",
  "## Before merge",
  "",
  "- [ ] Apply the migration",
  "  - Run: `make migrate`",
  "1. [x] Merge the branch — live as v2",
  "   1) [-] Roll back",
  "",
);

// Go reads the same cases, so the pane and the agent agree on every item.
test("the shared todo cases parse as Go parses them", () => {
  const url = new URL("../../../../../markdown/testdata/todo-items.json", import.meta.url);
  const cases = JSON.parse(readFileSync(url, "utf8"));
  for (const { name, text, items } of cases) {
    assert.deepEqual(
      parseTodo(text).items.map(({ section, text, note, state, line, where, check, run, blocks }) => ({ section, text, note, state, line, where, check, run, blocks })),
      items,
      name,
    );
  }
});

test("prose under an item stays apart from its fields, and the title and sections are named", () => {
  const parsed = parseTodo(doc("# Ship it", "", "## Now", "- [ ] Deploy", "  Wait for the canary.", "  - Where: prod", ""));
  assert.equal(parsed.title, "Ship it");
  assert.deepEqual(parsed.sections.map((section) => section.name), ["", "Now"]);
  assert.deepEqual(parsed.items[0].prose, ["Wait for the canary."]);
  assert.equal(parsed.items[0].to, 6);
  assert.equal(parsed.isTodo, true);
});

test("a document with no task items is not a todo list", () => {
  assert.equal(parseTodo(doc("# Notes", "", "- a plain bullet", "- [ ]", "")).isTodo, false);
});

// A splice that rebuilt the bullet would turn an ordered or nested item into another list.
test("a state change keeps the item's indentation, bullet and note", () => {
  const [, merge, rollback] = parseTodo(TODO).items;
  assert.equal(setState(TODO, merge, "open"), TODO.replace("1. [x] Merge the branch — live as v2", "1. [ ] Merge the branch — live as v2"));
  assert.equal(setState(TODO, rollback, "open"), TODO.replace("   1) [-] Roll back", "   1) [ ] Roll back"));
});

test("a note is written after a spaced em dash, replaced, or dropped when empty", () => {
  const [apply, merge] = parseTodo(TODO).items;
  assert.equal(setState(TODO, apply, "done", "ran it"), TODO.replace("- [ ] Apply the migration", "- [x] Apply the migration — ran it"));
  assert.equal(setState(TODO, merge, "done", "v3 now"), TODO.replace("live as v2", "v3 now"));
  assert.equal(setState(TODO, merge, "skipped", ""), TODO.replace("1. [x] Merge the branch — live as v2", "1. [-] Merge the branch"));
});

// A note holding the separator would move the split, and the item would read back with other text.
test("a written note reads back as the same item and note", () => {
  const [apply] = parseTodo(TODO).items;
  const next = setState(TODO, apply, "done", "one — two\nthree");
  assert.ok(next);
  const [again] = parseTodo(next).items;
  assert.equal(again.text, "Apply the migration");
  assert.equal(again.note, "one – two three");
});

test("a splice refuses a line that no longer holds the item", () => {
  const [apply] = parseTodo(TODO).items;
  assert.equal(setState(TODO.replace("Apply the migration", "Apply both migrations"), apply, "done"), null);
  assert.equal(setState(TODO.replace("- [ ] Apply the migration\n", "\n- [ ] Apply the migration"), apply, "done"), null);
});

test("pending changes reattach by section and text, and the rest detach", () => {
  const parsed = parseTodo(TODO);
  const [apply] = parsed.items;
  const kept = { key: todoKey(apply), section: apply.section, text: apply.text, state: /** @type {const} */ ("done"), note: "" };
  const gone = { key: todoKey({ section: "Before merge", text: "Old text", nth: 0 }), section: "Before merge", text: "Old text", state: /** @type {const} */ ("done"), note: "" };
  const laid = reapply(parsed, new Map([[kept.key, kept], [gone.key, gone]]));
  assert.equal(laid.items[0].draft, kept);
  assert.equal(laid.items[1].draft, null);
  assert.deepEqual(laid.detached, [gone]);
});

// The same text under another section is another item, so it must not take the draft.
test("a key names the section as well as the text", () => {
  assert.notEqual(todoKey({ section: "A", text: "Ship", nth: 0 }), todoKey({ section: "B", text: "Ship", nth: 0 }));
});

// Catches a PR shown twice, a bare #n linked to a guessed repository, or a tracked PR missing its state.
test("each pull request an item names is one GitHub link, tracked or not, and Where keeps only the rest", () => {
  const pulls = [{ repo: "acme/api", number: 12, title: "API" }];
  const api = { repo: "acme/api", number: 12, url: "https://github.com/acme/api/pull/12", pull: pulls[0] };
  const web = { repo: "acme/web", number: 7, url: "https://github.com/acme/web/pull/7", pull: null };
  assert.deepEqual(pullRefs({ where: "https://github.com/acme/api/pull/12", text: "Merge acme/api#12 then acme/web#7" }, pulls), {
    refs: [api, web],
    where: "",
  });
  assert.deepEqual(pullRefs({ where: "acme/web#7 (staging)", text: "Merge #12" }, pulls), { refs: [web], where: "(staging)" });
  assert.deepEqual(pullRefs({ where: "Phase 3 (acme/web#7)", text: "Merge" }, pulls), { refs: [web], where: "Phase 3" });
  assert.deepEqual(pullRefs({ where: "enclave · production", text: "Merge" }, null), { refs: [], where: "enclave · production" });
});

test("a run command loses one pair of backticks and nothing else", () => {
  assert.equal(runCommand("`make migrate`"), "make migrate");
  assert.equal(runCommand("``a` b``"), "`a` b`");
  assert.equal(runCommand("make migrate"), "make migrate");
  assert.equal(runCommand("`"), "`");
});

// Items sharing a section and text once shared a key, so ticking the second wrote the first.
test("ticking the second of two items with the same text writes the second line", () => {
  const text = doc("## Deploy", "- [ ] Restart the worker", "  - Where: eu", "- [ ] Restart the worker", "  - Where: us", "");
  const second = parseTodo(text).items[1];
  assert.equal(second.where, "us");
  const draft = { key: todoKey(second), section: second.section, text: second.text, state: /** @type {const} */ ("done") };
  assert.equal(spliceDraft(text, draft), text.replace("- [ ] Restart the worker\n  - Where: us", "- [x] Restart the worker\n  - Where: us"));
});

// A tick saved after the agent wrote a note must not wipe that note.
test("a tick with no typed note keeps the note on disk, and a typed one replaces it", () => {
  const [merge] = parseTodo(doc("- [ ] Merge", "")).items;
  const tick = { key: todoKey(merge), section: merge.section, text: merge.text, state: /** @type {const} */ ("done") };
  const agent = doc("- [x] Merge — live as v2.94.0", "");
  assert.equal(spliceDraft(agent, tick), agent);
  assert.equal(spliceDraft(agent, { ...tick, note: "" }), doc("- [x] Merge", ""));
  assert.equal(spliceDraft(agent, { ...tick, note: "shipped" }), doc("- [x] Merge — shipped", ""));
});
