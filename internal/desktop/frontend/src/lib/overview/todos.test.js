import assert from "node:assert/strict";
import test from "node:test";
import { todoTier } from "./todos.js";

const item = (text, line, path = "thoughts/shared/todos/T1-rollout.md") => ({ text, section: "Ship", where: "", path, line });

// Catches a sort that loses document order.
test("rows keep document order", () => {
  const rows = todoTier([item("Announce", 9), item("Migrate", 12), item("Tag", 14)]);
  assert.deepEqual(rows.map((row) => row.item.text), ["Announce", "Migrate", "Tag"]);
});

// Catches two documents' items on the same line sharing a row key.
test("rows are keyed by document and line", () => {
  const rows = todoTier([item("One", 9), item("Two", 9, "thoughts/shared/todos/T2-later.md")]);
  assert.deepEqual(rows.map((row) => row.key), ["thoughts/shared/todos/T1-rollout.md:9", "thoughts/shared/todos/T2-later.md:9"]);
});

// Catches the tier drawing an empty For you section.
test("no items, or none reported, make no rows", () => {
  assert.deepEqual(todoTier([]), []);
  assert.deepEqual(todoTier(undefined), []);
});
