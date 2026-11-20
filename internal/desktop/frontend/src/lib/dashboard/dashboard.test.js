import assert from "node:assert/strict";
import test from "node:test";
import { columnCount, estimatedHeight, guided, masonry, repoList } from "./dashboard.js";

test("the design's two window sizes get the columns it drew", () => {
  assert.equal(columnCount(1120), 3);
  assert.equal(columnCount(810), 2);
});

test("columns stay between one and four however wide or narrow the grid is", () => {
  assert.equal(columnCount(0), 1);
  assert.equal(columnCount(200), 1);
  assert.equal(columnCount(5000), 4);
  assert.equal(columnCount(NaN), 1);
});

// A count that ignored the UI scale would squeeze enlarged cards into columns too narrow for them.
test("a larger UI scale fits fewer columns into the same width", () => {
  assert.equal(columnCount(1120, 1), 3);
  assert.equal(columnCount(1120, 1.5), 2);
});

test("a card's estimate grows by one block per pull request it shows", () => {
  assert.equal(estimatedHeight(), 100);
  assert.equal(estimatedHeight(0), 100);
  assert.equal(estimatedHeight(3), 286);
});

// Placing by index alone would leave a tall card's column growing while its neighbours sat short.
test("each card goes to the column that is shortest when it is placed", () => {
  const heights = [estimatedHeight(4), estimatedHeight(0), estimatedHeight(0), estimatedHeight(0)];
  assert.deepEqual(masonry(heights, 2), [[0], [1, 2, 3]]);
});

// Ties broken anywhere but left would scramble the rail order across the first row.
test("equal cards fill the first row left to right, in rail order", () => {
  assert.deepEqual(masonry([100, 100, 100, 100, 100], 3), [[0, 3], [1, 4], [2]]);
});

test("one column holds every card in order, and no cards leave every column empty", () => {
  assert.deepEqual(masonry([100, 200, 100], 1), [[0, 1, 2]]);
  assert.deepEqual(masonry([], 3), [[], [], []]);
  assert.deepEqual(masonry([100], 0), [[0]]);
});

test("the repos line names every repository, and none says so", () => {
  assert.equal(repoList([{ name: "acme/web" }, { name: "acme/api" }]), "acme/web, acme/api");
  assert.equal(repoList([]), "No editing repositories");
});

test("only a guided session carries the RPI tag", () => {
  assert.equal(guided({ mode: "RPI" }), true);
  assert.equal(guided({ mode: "ASSISTANT" }), false);
  assert.equal(guided({}), false);
});
