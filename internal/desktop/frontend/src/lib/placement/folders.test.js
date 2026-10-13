import assert from "node:assert/strict";
import test from "node:test";
import { folderName } from "./folders.js";

test("a folder is named by its last path element", () => {
  assert.equal(folderName("/Users/me/Sync/lifesum"), "lifesum");
  assert.equal(folderName("/Users/me/Sync/lifesum/"), "lifesum");
  assert.equal(folderName("C:\\Sync\\kieranajp"), "kieranajp");
  assert.equal(folderName(""), "");
});
