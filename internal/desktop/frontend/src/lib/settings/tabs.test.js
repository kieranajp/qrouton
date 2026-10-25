import assert from "node:assert/strict";
import test from "node:test";
import { SETTINGS_FIELDS } from "../bridge/generated.js";
import { TABS, erroredTabs, firstErroredTab, stepTab, tabOf } from "./tabs.js";

test("every settings field Go names lives on a tab", () => {
  const keys = new Set(TABS.map((tab) => tab.key));
  for (const field of Object.values(SETTINGS_FIELDS)) {
    assert.ok(keys.has(tabOf(field) ?? ""), `${field} is on no tab`);
  }
  assert.equal(tabOf(`${SETTINGS_FIELDS.STICKERS}.3`), tabOf(SETTINGS_FIELDS.STICKERS));
  assert.equal(tabOf("unknown"), undefined);
});

test("ArrowDown visits every tab once before it returns", () => {
  /** @type {string[]} */
  const visited = [];
  let at = TABS[0].key;
  do {
    assert.ok(!visited.includes(at) && visited.length < TABS.length, `ArrowDown revisits ${at}`);
    visited.push(at);
    at = stepTab(at, "ArrowDown") ?? "";
  } while (at !== TABS[0].key);
  assert.deepEqual(visited, TABS.map((tab) => tab.key));
});

test("the first errored tab follows index order, not field order", () => {
  const fields = { launch: "not JSON", "stickers.0": "cannot be empty", root: "cannot be empty" };
  assert.deepEqual([...erroredTabs(fields)].sort(), ["advanced", "folders", "stickers"]);
  assert.equal(firstErroredTab(fields), "folders");
  assert.equal(firstErroredTab({}), undefined);
  assert.equal(firstErroredTab({ root: "" }), undefined);
});

test("arrow keys wrap through the index and Home and End reach its ends", () => {
  assert.equal(stepTab("general", "ArrowDown"), "github");
  assert.equal(stepTab("general", "ArrowUp"), "advanced");
  assert.equal(stepTab("advanced", "ArrowDown"), "general");
  assert.equal(stepTab("search", "Home"), "general");
  assert.equal(stepTab("search", "End"), "advanced");
  assert.equal(stepTab("search", "Enter"), undefined);
});
