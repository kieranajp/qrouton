import assert from "node:assert/strict";
import test from "node:test";
import { todoButtons } from "./todos.js";

const doc = (title, open, path = `thoughts/shared/todos/${title}.md`) => ({ path, title, open });

// Catches a button per open item instead of per document, or documents reordered.
test("one button per document, in the order given", () => {
  const buttons = todoButtons([doc("Rollout", 3), doc("Keys", 1)]);
  assert.deepEqual(buttons.map((button) => button.title), ["Rollout", "Keys"]);
  assert.deepEqual(buttons.map((button) => button.path), ["thoughts/shared/todos/Rollout.md", "thoughts/shared/todos/Keys.md"]);
});

// Catches the count wording drifting from the rail and dashboard, or a zero count drawn.
test("the count uses the rail's wording and is blank when nothing is open", () => {
  assert.deepEqual(todoButtons([doc("A", 1), doc("B", 2), doc("C", 0)]).map((button) => button.count), ["1 todo", "2 todos", ""]);
});

// Catches the row drawing for a session that reports no documents.
test("no documents, or none reported, make no buttons", () => {
  assert.deepEqual(todoButtons([]), []);
  assert.deepEqual(todoButtons(undefined), []);
});
