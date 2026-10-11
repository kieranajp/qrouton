import assert from "node:assert/strict";
import test from "node:test";
import { TABS, erroredTabs, firstErroredTab, stepTab, tabOf } from "./tabs.js";

test("every settings field lives on one tab", () => {
  assert.equal(tabOf("editor"), "general");
  assert.equal(tabOf("uiScale"), "general");
  assert.equal(tabOf("orgs"), "github");
  assert.equal(tabOf("root"), "folders");
  assert.equal(tabOf("thoughts"), "folders");
  assert.equal(tabOf("stickers.3"), "stickers");
  assert.equal(tabOf("launch"), "advanced");
  assert.equal(tabOf("unknown"), undefined);
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
  assert.deepEqual(
    TABS.map((tab) => tab.label),
    ["General", "GitHub", "Folders", "Search", "Stickers", "Advanced"],
  );
});
