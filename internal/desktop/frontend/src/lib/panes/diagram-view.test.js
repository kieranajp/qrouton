import assert from "node:assert/strict";
import { test } from "node:test";
import { clampTranslate, fitScale } from "./diagram-view.js";

test("a diagram wider than the box opens shrunk to fit across it", () => {
  assert.equal(fitScale({ width: 1000, height: 100 }, 500), 0.5);
  assert.equal(fitScale({ width: 200, height: 100 }, 50), 0.25);
});

test("a diagram the box already holds opens at the size it was emitted", () => {
  assert.equal(fitScale({ width: 180, height: 90 }, 180), 1);
  assert.equal(fitScale({ width: 180, height: 90 }, 900), 1);
});

test("a size or a box that states nothing leaves the diagram alone", () => {
  assert.equal(fitScale(null, 500), 1);
  assert.equal(fitScale({ width: 0, height: 0 }, 500), 1);
  assert.equal(fitScale({ width: 200, height: 100 }, 0), 1);
});

test("content the box holds is pinned flush against its leading edge", () => {
  assert.equal(clampTranslate(0, 500, 200), 0);
  assert.equal(clampTranslate(-40, 500, 200), 0);
  assert.equal(clampTranslate(120, 500, 200), 0);
});

test("content larger than the box cannot be dragged off either edge", () => {
  assert.equal(clampTranslate(-100, 500, 800), -100);
  assert.equal(clampTranslate(-500, 500, 800), -300);
  assert.equal(clampTranslate(60, 500, 800), 0);
  assert.equal(clampTranslate(Number.NaN, 500, 800), 0);
});
