import assert from "node:assert/strict";
import test from "node:test";
import {
  assemblyOpen,
  blocks,
  destination,
  folder,
  intent,
  joining,
  labels,
  last,
  pickerOpen,
  primary,
  refusal,
} from "./steps.js";

const NAME = { field: "name", message: "A name is needed." };
const REPOS = { field: "repos", message: "At least one editing repo is needed." };

test("each step names itself and its own way forward", () => {
  assert.equal(labels.length, 3);
  assert.equal(primary(0), "Choose repositories →");
  assert.equal(primary(last), "Create session →");
});

// Step 1 has nothing on screen to pick a repository with.
test("a step is only stopped by a problem it can be fixed on", () => {
  assert.equal(blocks([REPOS], 0), undefined);
  assert.deepEqual(blocks([REPOS], 1), REPOS);
  assert.deepEqual(blocks([NAME], 0), NAME);
  assert.equal(blocks([NAME], 1), undefined);
});

test("the last step is stopped by anything, wherever it was left", () => {
  assert.deepEqual(blocks([NAME, REPOS], last), NAME);
  assert.deepEqual(blocks([REPOS], last), REPOS);
  assert.equal(blocks([], last), undefined);
});

test("the folder a branch names is the branch without its prefix", () => {
  assert.equal(folder("feat/extract-billing-service"), "extract-billing-service");
  assert.equal(folder("/extract-billing-service"), "extract-billing-service");
  assert.equal(folder(""), "");
});

test("the destination counts repositories and names the folder", () => {
  assert.equal(destination("feat/billing", 3), "3 repos into billing");
  assert.equal(destination("feat/billing", 1), "1 repo into billing");
  assert.equal(destination("", 3), "");
});

test("the picker names the branch it adds to, and a branchless session says nothing", () => {
  assert.equal(joining("feat/billing"), "Added repositories join feat/billing");
  assert.equal(joining(""), "");
});

test("a window holding no session is the assembly overlay", () => {
  assert.equal(assemblyOpen(false, true, ""), true);
  assert.equal(assemblyOpen(false, true, "billing"), false);
  assert.equal(assemblyOpen(true, true, "billing"), true);
});

// An unsettled window has an empty slug too, and flashing assembly over a real
// session is worse than opening it a moment late.
test("assembly waits for the first payload before deciding there is no session", () => {
  assert.equal(assemblyOpen(false, false, ""), false);
  assert.equal(assemblyOpen(true, false, ""), false);
  assert.equal(assemblyOpen(true, false, "billing"), false);
});

test("the picker is open for an escalation waiting on the session on screen", () => {
  assert.equal(pickerOpen("billing", true, ""), true);
  assert.equal(pickerOpen("billing", false, ""), false);
  assert.equal(pickerOpen("", true, ""), false);
});

// Add-repos belongs to the session it was pressed on, so switching away closes it.
test("add-repos is open only over the session it was pressed on", () => {
  assert.equal(pickerOpen("billing", false, "billing"), true);
  assert.equal(pickerOpen("webhooks", false, "billing"), false);
});

test("a refusal reads as the sentence, without the field Go named it by", () => {
  assert.equal(
    refusal(new Error("repos: At least one editing repo is needed.")),
    "At least one editing repo is needed.",
  );
  const unprefixed = 'no session named "billing" under the sessions root';
  assert.equal(refusal(unprefixed), unprefixed);
  assert.equal(refusal(undefined), "");
});

test("Enter advances, and a newline in the description stays a newline", () => {
  assert.equal(intent({ key: "Enter", target: { tagName: "INPUT" } }), "advance");
  assert.equal(intent({ key: "Enter", target: { tagName: "textarea" } }), "");
  assert.equal(intent({ key: "Enter", target: { tagName: "BUTTON" } }), "");
});

test("Escape cancels from anywhere, and nothing else is bound", () => {
  assert.equal(intent({ key: "Escape", target: { tagName: "TEXTAREA" } }), "cancel");
  assert.equal(intent({ key: "Tab", target: { tagName: "INPUT" } }), "");
  assert.equal(intent(), "");
});
