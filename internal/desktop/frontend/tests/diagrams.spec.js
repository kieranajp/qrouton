import { expect, test } from "@playwright/test";

test("a fence waiting on its diagram says so, then draws it inside the measure", async ({ page }) => {
  await page.goto("/tests/diagrams.html");

  const fence = await page.evaluate(() => window.probe());
  expect(fence.found).toBe(true);
  expect(fence.line).toBe("3");

  const waiting = await page.evaluate(() => (window.pending(), window.probe()));
  expect(waiting.pending).toBe(true);
  expect(waiting.code).toBe(true);
  expect(waiting.opacity).toBeLessThan(1);

  const drawn = await page.evaluate(() => (window.draw(), window.probe()));
  expect(drawn.pending).toBe(false);
  expect(drawn.drawn).toBe(true);
  expect(drawn.code).toBe(false);
  // The line survives the swap: the gutter number and the viewport measure by it.
  expect(drawn.line).toBe("3");
  expect(drawn.lineEnd).toBe("6");
  expect(drawn.gutter).toBe('"3"');
  expect(Number.parseFloat(drawn.gutterLine)).toBeGreaterThan(0);

  expect(drawn.width).toBeGreaterThan(0);
  expect(drawn.height).toBeGreaterThan(0);
  // Wider than the measure, so the block scrolls rather than the page.
  expect(drawn.scrolls).toBe(true);
  expect(drawn.block).toBeLessThanOrEqual(drawn.container);
  expect(drawn.page).toBeLessThanOrEqual(drawn.viewport);
});

test("a diagram that failed to render leaves the fence as code", async ({ page }) => {
  await page.goto("/tests/diagrams.html");
  const failed = await page.evaluate(() => (window.pending(), window.fail(), window.probe()));

  expect(failed.pending).toBe(false);
  expect(failed.drawn).toBe(false);
  expect(failed.code).toBe(true);
});
