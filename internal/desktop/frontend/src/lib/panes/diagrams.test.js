import assert from "node:assert/strict";
import { test } from "node:test";
import { naturalSize, place } from "./diagrams.js";

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
