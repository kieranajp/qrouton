import assert from "node:assert/strict";
import test from "node:test";
import {
  assemblyGated,
  assemblyOpen,
  blocks,
  destination,
  folder,
  intent,
  joining,
  labels,
  last,
  pickerKey,
  pickerOpen,
  primary,
  refusal,
} from "./steps.js";
import { DRAFT_FIELDS } from "../bridge/generated.js";

const NAME = { field: DRAFT_FIELDS.NAME, message: "A name is needed." };
const BRANCH = { field: DRAFT_FIELDS.BRANCH_DESCRIPTION, message: "That folder already exists." };
const REPOS = { field: DRAFT_FIELDS.REPOS, message: "At least one editing repo is needed." };

test("each step names itself and its own way forward", () => {
  assert.equal(labels.length, 3);
  assert.equal(primary(0), "Choose repositories");
  assert.equal(primary(last), "Create session");
});

// Step 1 has nothing on screen to pick a repository with.
test("a step is only stopped by a problem it can be fixed on", () => {
  assert.equal(blocks([REPOS], 0), undefined);
  assert.deepEqual(blocks([REPOS], 1), REPOS);
  assert.deepEqual(blocks([NAME], 0), NAME);
  assert.deepEqual(blocks([BRANCH], 0), BRANCH);
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

test("an install with no sessions is the assembly overlay", () => {
  assert.equal(assemblyOpen(false, true, "", 0), true);
  assert.equal(assemblyOpen(false, true, "billing", 1), false);
  assert.equal(assemblyOpen(true, true, "billing", 1), true);
});

// With no session shown the window is the dashboard, and the overlay opening unasked would cover it.
test("the dashboard with sessions on disk does not raise the overlay until asked", () => {
  assert.equal(assemblyOpen(false, true, "", 3), false);
  assert.equal(assemblyOpen(true, true, "", 3), true);
});

// A session created from the gated overlay shows before the rail lists it.
test("a shown session the rail has not listed yet does not reopen the overlay", () => {
  assert.equal(assemblyOpen(false, true, "billing", 0), false);
});

// An unsettled window has no sessions too, and flashing assembly over a real
// session is worse than opening it a moment late.
test("assembly waits for the first payload before deciding there is no session", () => {
  assert.equal(assemblyOpen(false, false, "", 0), false);
  assert.equal(assemblyOpen(true, false, "", 0), false);
  assert.equal(assemblyOpen(true, false, "billing", 1), false);
});

// Gated with sessions present, Escape on the first step would trap the user instead of returning to the dashboard.
test("the overlay is gated only while no session exists", () => {
  assert.equal(assemblyGated("", 0), true);
  assert.equal(assemblyGated("", 2), false);
  assert.equal(assemblyGated("billing", 0), false);
});

test("the picker is open for an escalation waiting on the session on screen", () => {
  assert.equal(pickerOpen("", "billing", "e1"), true);
  assert.equal(pickerOpen("", "billing", ""), false);
  assert.equal(pickerOpen("", "", "e1"), false);
});

// Add-repos belongs to the session it was pressed on, so switching away closes it.
test("add-repos is open only over the session it was pressed on", () => {
  assert.equal(pickerOpen("billing", "billing", ""), true);
  assert.equal(pickerOpen("billing", "webhooks", ""), false);
});

// Keyed on the escalation, a mid-add arrival would remount the running add as an Escalate.
test("the overlay stays with a running add until it ends, then answers the escalation", () => {
  assert.equal(pickerKey("billing", "billing", ""), "add");
  assert.equal(pickerKey("billing", "billing", "e1"), "add");
  assert.equal(pickerKey("", "billing", "e1"), "e1");
  assert.equal(pickerKey("", "billing", "e2"), "e2");
  assert.equal(pickerKey("", "billing", ""), "");
  assert.equal(pickerKey("billing", "webhooks", "e1"), "e1");
});

test("a refusal reads as the sentence, without the field Go named it by", () => {
  assert.equal(
    refusal(new Error("repos: At least one editing repo is needed.")),
    "At least one editing repo is needed.",
  );
  assert.equal(
    refusal(new Error("branchDescription: That folder already exists.")),
    "That folder already exists.",
  );
  const unprefixed = 'no session named "billing" under the sessions root';
  assert.equal(refusal(unprefixed), unprefixed);
  assert.equal(refusal(undefined), "");
  assert.equal(refusal("unknown: text"), "unknown: text");
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
