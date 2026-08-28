import assert from "node:assert/strict";
import { test } from "node:test";
import { clampScale, clampTranslate, fitScale, panBy, stepScale, zoomAt } from "./diagram-view.js";

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

test("zooming holds the content under the pointer still", () => {
  for (const scale of [0.34, 1, 2.5]) {
    for (const next of [0.5, 1.2, 8]) {
      for (const at of [{ x: 0, y: 0 }, { x: 210, y: 44 }, { x: 4000, y: -30 }]) {
        const state = { scale, tx: -60, ty: -12 };
        const zoomed = zoomAt(state, at, next);
        assert.ok(
          Math.abs((at.x - zoomed.tx) / zoomed.scale - (at.x - state.tx) / state.scale) < 1e-9,
          `x at ${at.x} from ${scale} to ${next}`,
        );
        assert.ok(
          Math.abs((at.y - zoomed.ty) / zoomed.scale - (at.y - state.ty) / state.scale) < 1e-9,
          `y at ${at.y} from ${scale} to ${next}`,
        );
      }
    }
  }
});

test("a scale that states nothing leaves the view where it was", () => {
  assert.deepEqual(zoomAt({ scale: 1, tx: -5, ty: -5 }, { x: 10, y: 10 }, 0), {
    scale: 1,
    tx: -5,
    ty: -5,
  });
  assert.deepEqual(zoomAt({ scale: 0, tx: -5, ty: -5 }, { x: 10, y: 10 }, 2), {
    scale: 0,
    tx: -5,
    ty: -5,
  });
});

test("a step doubles or halves over two clicks and stops at either end", () => {
  assert.equal(stepScale(1, 1, 0.25), Math.SQRT2);
  assert.ok(Math.abs(stepScale(stepScale(1, 1, 0.25), 1, 0.25) - 2) < 1e-12);
  assert.ok(Math.abs(stepScale(stepScale(1, -1, 0.25), -1, 0.25) - 0.5) < 1e-12);
  assert.equal(stepScale(0.3, -1, 0.25), 0.25);
  assert.equal(stepScale(7, 1, 0.25), 8);
  assert.equal(stepScale(8, 1, 0.25), 8);
});

test("the fitted scale is the floor and eight hundred percent the ceiling", () => {
  assert.equal(clampScale(0.1, 0.34), 0.34);
  assert.equal(clampScale(400, 0.34), 8);
  assert.equal(clampScale(Number.NaN, 0.34), 0.34);
  assert.equal(clampScale(2, 0.34), 2);
});

const box = { width: 400, height: 200 };

test("a drag moves the view by the pointer and stops at the edges", () => {
  const content = { width: 1000, height: 500 };
  assert.deepEqual(panBy({ tx: -100, ty: -50, button: 0 }, { x: -30, y: -20 }, box, content), {
    tx: -130,
    ty: -70,
  });
  assert.deepEqual(panBy({ tx: -100, ty: -50, button: 0 }, { x: 400, y: 400 }, box, content), {
    tx: 0,
    ty: 0,
  });
  assert.deepEqual(panBy({ tx: -100, ty: -50, button: 0 }, { x: -900, y: -900 }, box, content), {
    tx: -600,
    ty: -300,
  });
});

test("a drag at the fitted default has nothing to move", () => {
  const content = { width: 400, height: 200 };
  for (const by of [{ x: -80, y: -40 }, { x: 80, y: 40 }]) {
    assert.deepEqual(panBy({ tx: 0, ty: 0, button: 0 }, by, box, content), { tx: 0, ty: 0 });
  }
});

test("a drag on any button but the primary one is not a pan", () => {
  const content = { width: 1000, height: 500 };
  for (const button of [1, 2, -1]) {
    assert.deepEqual(panBy({ tx: -100, ty: -50, button }, { x: -30, y: -20 }, box, content), {
      tx: -100,
      ty: -50,
    });
  }
});
