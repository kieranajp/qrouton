import assert from "node:assert/strict";
import { test } from "node:test";
import { counterFor, partition, screenFor } from "./deck.js";
import { parsePlan } from "./plan.js";

const lines = (...text) => text.join("\n");

const PLAN = lines(
  "---", // 1
  "kind: plan", // 2
  "---", // 3
  "", // 4
  "# The deck", // 5
  "", // 6
  "Preamble prose.", // 7
  "", // 8
  "## Phase 1 — Groundwork", // 9
  "", // 10
  "Groundwork body.", // 11
  "", // 12
  "### Verify", // 13
  "- [x] one", // 14
  "- [ ] two", // 15
  "", // 16
  "## Phase 2 — The middle", // 17
  "", // 18
  "Middle body.", // 19
  "", // 20
  "### Verify", // 21
  "- [ ] later", // 22
  "", // 23
  "## Blockers", // 24
  "", // 25
  "None.", // 26
  "",
);

const plan = parsePlan(PLAN);

/** A rendered block as the renderer deals it out: its markup and its source span. */
const block = (from, to = from) => ({ html: `<p data-line="${from}">${from}</p>`, from, to });
const dealing =
  (...blocks) =>
  () => blocks;
const html = (...blocks) => blocks.map((one) => one.html).join("");

const opening = block(9);
const body = block(11);
const heading = block(13);
const checks = block(14, 15);

test("every block lands in the slide whose span holds it", () => {
  const before = [block(5), block(7)];
  const middle = [block(17), block(19), block(21), block(22)];
  const section = [block(24), block(26)];
  const deck = partition(
    "",
    plan,
    dealing(...before, opening, body, heading, checks, ...middle, ...section),
  );

  assert.equal(deck.preamble, html(...before));
  assert.deepEqual(deck.slides, [
    { opening: html(opening), body: html(body), criteria: html(heading, checks) },
    { opening: html(middle[0]), body: html(middle[1]), criteria: html(middle[2], middle[3]) },
    { opening: html(section[0]), body: html(section[1]), criteria: "" },
  ]);
});

test("a block the parser numbered nowhere is bucketed with the block before it", () => {
  const inBody = { html: "<figure>drawn</figure>", from: body.from, to: body.to };
  const inCriteria = { html: "<figure>ticked</figure>", from: checks.from, to: checks.to };
  const deck = partition("", plan, dealing(opening, body, inBody, heading, checks, inCriteria));

  assert.equal(deck.slides[0].body, html(body, inBody));
  assert.equal(deck.slides[0].criteria, html(heading, checks, inCriteria));
});

test("a block ending past the criteria it opens in stays in the body", () => {
  const spilling = block(14, 16);
  const deck = partition("", plan, dealing(opening, heading, spilling));

  assert.equal(deck.slides[0].criteria, html(heading));
  assert.equal(deck.slides[0].body, html(spilling));
});

test("the criteria of one phase claim nothing in the next", () => {
  const straddling = /** @type {any} */ ({
    slides: [
      { from: 9, to: 16, number: 1, name: "Groundwork", verify: { from: 13, to: 19 } },
      { from: 17, to: 20, number: 2, name: "The middle", verify: null },
    ],
  });
  const after = block(19);
  const deck = partition("", straddling, dealing(heading, checks, block(17), after));

  assert.equal(deck.slides[0].criteria, html(heading, checks));
  assert.equal(deck.slides[1].body, html(after));
});

test("a line in no slide reads as the overview", () => {
  assert.equal(screenFor(plan.slides, 0), 0);
  assert.equal(screenFor(plan.slides, 7), 0);
  assert.equal(screenFor(plan.slides, 999), 0);
});

test("a line inside a slide reads as the screen after the overview", () => {
  assert.equal(screenFor(plan.slides, 9), 1);
  assert.equal(screenFor(plan.slides, 15), 1);
  assert.equal(screenFor(plan.slides, 19), 2);
  assert.equal(screenFor(plan.slides, 26), 3);
});

test("a phase counts in phases and a section answers with its name", () => {
  assert.equal(counterFor(plan, 0), "Overview");
  assert.equal(counterFor(plan, 1), "1 / 2");
  assert.equal(counterFor(plan, 2), "2 / 2");
  assert.equal(counterFor(plan, 3), "Blockers");
});
