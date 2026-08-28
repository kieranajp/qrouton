import assert from "node:assert/strict";
import { test } from "node:test";
import { apply, emittedSize, naturalSize, place } from "./diagrams.js";

const block = (line) => ({ dataset: { line: String(line) } });

test("a result is placed on the block its line names", () => {
  const blocks = [block(3), block(11)];
  const placed = place(blocks, [{ line: 11, svg: "<svg/>" }, { line: 3 }]);

  assert.deepEqual(
    placed.map(({ block: found, result }) => [blocks.indexOf(found), result.line]),
    [
      [1, 11],
      [0, 3],
    ],
  );
});

test("a result no block carries is dropped rather than misplaced", () => {
  assert.deepEqual(place([block(3)], [{ line: 4 }]), []);
  assert.deepEqual(place([], [{ line: 3 }]), []);
  assert.deepEqual(place([block(3)], []), []);
});

test("a block with no source line matches nothing", () => {
  assert.deepEqual(place([{ dataset: {} }], [{ line: 0 }]), []);
});

test("a viewBox gives the diagram its own size", () => {
  assert.deepEqual(naturalSize("0 0 1642 108"), { width: 1642, height: 108 });
  assert.deepEqual(naturalSize("0,0,120,60"), { width: 120, height: 60 });
});

test("a viewBox that names no size is left to the browser", () => {
  for (const box of [null, "", "0 0 1642", "0 0 a b", "0 0 0 108"]) {
    assert.equal(naturalSize(box), null, `viewBox ${JSON.stringify(box)}`);
  }
});

const svg = (/** @type {Record<string, string>} */ attributes) => ({
  getAttribute: (/** @type {string} */ name) => attributes[name] ?? null,
});

test("the size the renderer emitted beats the viewBox it left at natural size", () => {
  const drawn = svg({ width: "1067", height: "70", viewBox: "0 0 1642 108" });
  assert.deepEqual(emittedSize(drawn), { width: 1067, height: 70 });
});

test("output carrying no size of its own is taken down to the renderer's ceiling", () => {
  assert.deepEqual(emittedSize(svg({ viewBox: "0 0 200 100" })), { width: 130, height: 65 });
  assert.deepEqual(emittedSize(svg({ width: "200", viewBox: "0 0 200 100" })), {
    width: 130,
    height: 65,
  });
});

test("a size that states nothing usable falls back rather than drawing at it", () => {
  for (const size of [{ width: "0", height: "0" }, { width: "-4", height: "-1" }, { width: "wide", height: "tall" }]) {
    assert.deepEqual(emittedSize(svg({ ...size, viewBox: "0 0 200 100" })), {
      width: 130,
      height: 65,
    });
    assert.equal(emittedSize(svg(size)), null, JSON.stringify(size));
  }
});

test("a size given in anything but pixels is not a pixel count", () => {
  for (const width of ["100%", "12em", "3pt", "1e3"]) {
    assert.deepEqual(emittedSize(svg({ width, height: width, viewBox: "0 0 200 100" })), {
      width: 130,
      height: 65,
    });
  }
  assert.deepEqual(emittedSize(svg({ width: "180px", height: "90px" })), { width: 180, height: 90 });
});

test("nothing readable leaves the size to the browser", () => {
  assert.equal(emittedSize(svg({})), null);
  assert.equal(emittedSize(null), null);
});

// Enough of a <pre> for apply to settle it; the drawn geometry is a browser
// question and is asserted against a real one.
const fence = () => {
  const classes = new Set();
  /** @type {{className: string, textContent: string}[]} */
  const children = [];
  return {
    children,
    classList: {
      add: (/** @type {string[]} */ ...names) => names.forEach((name) => classes.add(name)),
      remove: (/** @type {string[]} */ ...names) => names.forEach((name) => classes.delete(name)),
      contains: (/** @type {string} */ name) => classes.has(name),
    },
    dataset: { line: "3" },
    ownerDocument: { createElement: () => ({ className: "", textContent: "" }) },
    append: (/** @type {any} */ child) => children.push(child),
    querySelector: (/** @type {string} */ selector) =>
      children.find((child) => `.${child.className}` === selector) ?? null,
  };
};

const failed = (block) => /** @type {any} */ ({ querySelectorAll: () => [block] });

test("a fence that failed states its reason as text, once", () => {
  const block = fence();
  const container = failed(block);

  apply(container, [{ line: 3, error: "5:1: <b> is not a shape" }]);
  apply(container, [{ line: 3, error: "5:1: <b> is not a shape" }]);

  assert.equal(block.children.length, 1);
  assert.equal(block.children[0].textContent, "5:1: <b> is not a shape");
  assert.equal(block.classList.contains("diagram-failed"), true);
  assert.equal(block.classList.contains("diagram-pending"), false);
});

test("a reply arriving after the failure does not put the fence back to waiting", () => {
  const block = fence();
  const container = failed(block);

  apply(container, [{ line: 3, error: "diagram took too long to lay out" }]);
  apply(container, [{ line: 3 }]);

  assert.equal(block.classList.contains("diagram-pending"), false);
  assert.equal(block.classList.contains("diagram-failed"), true);
});

test("a fence that had already drawn loses that class when it later fails", () => {
  const block = fence();
  block.classList.add("diagram");
  const container = failed(block);

  apply(container, [{ line: 3, error: "5:1: <b> is not a shape" }]);

  assert.equal(block.classList.contains("diagram"), false);
  assert.equal(block.classList.contains("diagram-failed"), true);
});
