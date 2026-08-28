import { expect, test } from "@playwright/test";

const latest = (page) => page.evaluate(() => window.reports.at(-1));
const waitForReport = (page, count) =>
  page.waitForFunction((minimum) => window.reports.length >= minimum, count);

test("partially intersecting blocks report their complete source intervals", async ({ page }) => {
  await page.goto("/tests/viewport.html");
  await waitForReport(page, 1);
  await page.evaluate(() => window.scrollToTargetEdges());
  await page.waitForFunction(() => window.reports.at(-1)?.intervals?.[0]?.line === 3);
  await expect.poll(() => latest(page)).toEqual({
    seq: expect.any(Number),
    available: true,
    selected: true,
    intervals: [
      { line: 3, to: 5 },
      { line: 8, to: 8 },
    ],
  });
});

test("a viewport containing no stamped block is measured empty", async ({ page }) => {
  await page.goto("/tests/viewport.html");
  await waitForReport(page, 1);
  await page.evaluate(() => window.scrollToGap());
  await page.waitForFunction(() => window.reports.at(-1)?.available && window.reports.at(-1)?.intervals.length === 0);
  await expect.poll(() => latest(page)).toEqual({
    seq: expect.any(Number),
    available: true,
    selected: true,
    intervals: [],
  });
});

test("a hidden scroll root is unavailable", async ({ page }) => {
  await page.goto("/tests/viewport.html");
  await waitForReport(page, 1);
  await page.locator("#root").evaluate((root) => (root.style.display = "none"));
  await page.evaluate(() => window.measure());
  await page.waitForFunction(() => window.reports.at(-1)?.available === false);
  await expect.poll(() => latest(page)).toEqual({
    seq: expect.any(Number),
    available: false,
    selected: true,
    intervals: [],
  });
});

test("inactive documents report unselected and unavailable", async ({ page }) => {
  await page.goto("/tests/viewport.html");
  await waitForReport(page, 1);
  await page.evaluate(() => window.setSelected(false));
  await expect.poll(() => latest(page)).toEqual({
    seq: expect.any(Number),
    available: false,
    selected: false,
    intervals: [],
  });
});

test("content mounted under a hidden ancestor reveals after activation", async ({ page }) => {
  await page.goto("/tests/viewport.html?hidden=true&selected=false");
  await waitForReport(page, 1);
  await expect.poll(() => latest(page)).toMatchObject({ available: false, selected: false });
  await page.evaluate(() => window.activate());
  await expect.poll(() => page.evaluate(() => window.reveals)).toBe(1);
  await expect.poll(() => latest(page)).toMatchObject({
    available: true,
    selected: true,
    intervals: expect.arrayContaining([{ line: 3, to: 5 }]),
  });
});

test("selection arriving after content mount triggers the measured reveal", async ({ page }) => {
  await page.goto("/tests/viewport.html?selected=false");
  await waitForReport(page, 1);
  await page.evaluate(() => window.setSelected(true));
  await expect.poll(() => page.evaluate(() => window.reveals)).toBe(1);
  await expect.poll(() => latest(page)).toMatchObject({
    available: true,
    selected: true,
    intervals: expect.arrayContaining([{ line: 3, to: 5 }]),
  });
});

test("a line inside a multiline block reports the block's full source interval", async ({ page }) => {
  await page.goto("/tests/viewport.html");
  await expect.poll(() => latest(page)).toMatchObject({
    available: true,
    selected: true,
    intervals: expect.arrayContaining([{ line: 3, to: 5 }]),
  });
});

test("overlapping nested intervals merge in source order", async ({ page }) => {
  await page.goto("/tests/viewport.html");
  await waitForReport(page, 1);
  await page.evaluate(() => window.scrollToNested());
  await expect.poll(() => latest(page)).toEqual({
    seq: expect.any(Number),
    available: true,
    selected: true,
    intervals: [{ line: 20, to: 24 }],
  });
});

test("coalesced font, content, and root resize recovers a displaced target once", async ({ page }) => {
  await page.goto("/tests/viewport.html");
  await expect.poll(() => page.evaluate(() => window.reveals)).toBe(1);
  await page.waitForTimeout(50);
  await page.evaluate(() => window.moveTargetWithReflow());
  await expect.poll(() => page.evaluate(() => window.reveals)).toBe(2);
  await expect.poll(() => latest(page)).toMatchObject({
    available: true,
    selected: true,
    intervals: expect.arrayContaining([{ line: 3, to: 5 }]),
  });
  await page.waitForTimeout(100);
  expect(await page.evaluate(() => window.reveals)).toBe(2);
});

test("a resize does not undo a later manual scroll", async ({ page }) => {
  await page.goto("/tests/viewport.html");
  await expect.poll(() => page.evaluate(() => window.reveals)).toBe(1);
  await page.evaluate(() => window.scrollToNested());
  await expect.poll(() => latest(page)).toMatchObject({ intervals: [{ line: 20, to: 24 }] });
  await page.evaluate(() => window.resizeAfterScroll());
  await page.waitForTimeout(100);
  expect(await page.evaluate(() => window.reveals)).toBe(1);
  await expect.poll(() => latest(page)).not.toMatchObject({
    intervals: expect.arrayContaining([{ line: 3, to: 5 }]),
  });
});

test("deactivation publishes unavailable state and cancels stale scheduled work", async ({ page }) => {
  await page.goto("/tests/viewport.html");
  await expect.poll(() => page.evaluate(() => window.reveals)).toBe(1);
  const before = await page.evaluate(() => ({ reports: window.reports.length, reveals: window.reveals }));
  await page.evaluate(() => {
    window.setSelected(false);
    window.setSelected(true);
    window.setSelected(false);
  });
  await expect.poll(() => latest(page)).toMatchObject({ available: false, selected: false, intervals: [] });
  await page.waitForTimeout(100);
  expect(await page.evaluate(() => window.reveals)).toBe(before.reveals);
  expect(await page.evaluate(() => window.reports.length)).toBe(before.reports + 1);
});

// The load-bearing one: Go claims a scroll succeeded from these intervals, and
// they are measured off block elements captured at mount. A diagram arriving a
// microtask later must leave that array live and the stamped lines intact.
test("a diagram swapped in keeps the lines around it measurable", async ({ page }) => {
  await page.goto("/tests/viewport.html");
  await waitForReport(page, 1);
  await page.evaluate(() => window.scrollToDiagram());
  await page.waitForFunction(() => window.reports.at(-1)?.intervals?.[0]?.line === 30);
  await expect.poll(() => latest(page)).toMatchObject({
    available: true,
    intervals: [
      { line: 30, to: 30 },
      { line: 32, to: 35 },
      { line: 37, to: 37 },
    ],
  });

  const code = await page.evaluate(() => window.diagramHeight());
  await page.evaluate(() => window.drawDiagram());
  await expect.poll(() => page.evaluate(() => window.diagramHeight())).toBeGreaterThan(code);

  // Re-measured off the same elements: the taller diagram now fills the
  // viewport and has pushed the prose under it out of view.
  await expect.poll(() => latest(page)).toMatchObject({
    available: true,
    intervals: [
      { line: 30, to: 30 },
      { line: 32, to: 35 },
    ],
  });

  await page.evaluate(() => window.scrollPastDiagram());
  await expect.poll(() => latest(page)).toMatchObject({
    available: true,
    intervals: [
      { line: 32, to: 35 },
      { line: 37, to: 37 },
    ],
  });
});
