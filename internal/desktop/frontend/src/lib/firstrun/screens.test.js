import assert from "node:assert/strict";
import test from "node:test";
import { back, blocking, caps, last, pip, primary, total } from "./screens.js";

test("there are five screens, three of them explaining and two asking", () => {
  assert.equal(total, 5);
  assert.equal(last, 4);
  assert.equal(caps(1), "The one idea to know");
  assert.equal(caps(3), "Question 1 of 2");
  assert.equal(caps(4), "Question 2 of 2");
});

// First run is a gate: there is nowhere to go back to from the first screen.
test("only the first screen has nothing to go back to", () => {
  assert.equal(back(0), "");
  for (const step of [1, 2, 3, 4]) assert.equal(back(step), "← Back");
});

test("each screen names its own way forward", () => {
  assert.equal(primary(0), "Show me →");
  assert.equal(primary(1), "Next →");
  assert.equal(primary(2), "Set it up →");
  assert.equal(primary(3), "Next →");
  assert.equal(primary(last), "Find my repositories →");
});

test("the lit pip is the step, and a step outside the five still lights one", () => {
  assert.equal(pip(0), 0);
  assert.equal(pip(4), 4);
  assert.equal(pip(9), 4);
  assert.equal(pip(-1), 0);
});

test("a step outside the five falls back to the first screen's chrome", () => {
  assert.equal(primary(9), "Show me →");
  assert.equal(back(9), "");
});

// Answering with no owners would write the welcomed marker, and a workbench with
// no sessions has no route back to the question or to Settings.
test("the owners question cannot be left unanswered", () => {
  assert.equal(blocking(3, [], ""), "Add at least one organisation or username to search.");
});

test("a committed chip answers the owners question", () => {
  assert.equal(blocking(3, ["acme"], ""), "");
});

// Next commits what is in the field on the way out, so a typed owner is an
// answer already.
test("an owner typed but not yet a chip answers the owners question", () => {
  assert.equal(blocking(3, [], "acme"), "");
});

test("whitespace in the field answers nothing", () => {
  assert.equal(blocking(3, [], "   "), "Add at least one organisation or username to search.");
});

test("no other screen has an answer it will not accept, nor does a step outside the five", () => {
  for (const step of [0, 1, 2, 4, 9]) assert.equal(blocking(step, [], ""), "");
});
