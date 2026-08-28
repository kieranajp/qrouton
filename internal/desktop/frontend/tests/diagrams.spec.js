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

test("a diagram that failed to render leaves the fence as code and says why", async ({ page }) => {
  await page.goto("/tests/diagrams.html");
  const failed = await page.evaluate(() => (window.pending(), window.fail(), window.probe()));

  expect(failed.pending).toBe(false);
  expect(failed.drawn).toBe(false);
  expect(failed.code).toBe(true);
  expect(failed.failed).toBe(true);
  expect(failed.error).toBe("diagram took too long to lay out");
  expect(failed.stated).toBeGreaterThan(0);
  // A finished failure, not a fence still dimmed as though something is coming.
  expect(failed.opacity).toBe(1);
});

test("a good fence, a broken one and an unsupported one settle into three visible outcomes", async ({ page }) => {
  await page.goto("/tests/diagrams.html");
  const lines = await page.evaluate(() => window.lines);
  const probe = (line) => page.evaluate((at) => window.probe(at), line);
  await page.evaluate(() => (window.pending(), window.settle()));

  const drawn = await probe(lines.drawn);
  expect(drawn.drawn).toBe(true);
  expect(drawn.height).toBeGreaterThan(0);
  expect(drawn.error).toBe("");

  const broken = await probe(lines.broken);
  const embedded = await probe(lines.embedded);
  for (const failed of [broken, embedded]) {
    expect(failed.failed).toBe(true);
    expect(failed.drawn).toBe(false);
    expect(failed.pending).toBe(false);
    expect(failed.code).toBe(true);
    expect(failed.stated).toBeGreaterThan(0);
    expect(failed.tall).toBeGreaterThan(0);
  }
  expect(broken.error).toContain("11:5");
  expect(embedded.error).toContain("|md|");
  expect(broken.error).not.toBe(embedded.error);
});

test("a reason quoting the document's own markup is read as text", async ({ page }) => {
  await page.goto("/tests/diagrams.html");
  const quoted = '<img src=x onerror="boom()">: not a shape';
  const failed = await page.evaluate((message) => {
    window.fail(window.lines.broken, message);
    return window.probe(window.lines.broken);
  }, quoted);

  expect(failed.error).toBe(quoted);
  expect(failed.markup).toBe(0);
});

test("a reply naming every fence does not put a settled failure back to waiting", async ({ page }) => {
  await page.goto("/tests/diagrams.html");
  const failed = await page.evaluate(() => {
    window.fail(window.lines.broken, "11:5: unexpected end of file");
    window.pending();
    window.fail(window.lines.broken, "11:5: unexpected end of file");
    return window.probe(window.lines.broken);
  });

  expect(failed.pending).toBe(false);
  expect(failed.failed).toBe(true);
  expect(failed.notes).toBe(1);
});
