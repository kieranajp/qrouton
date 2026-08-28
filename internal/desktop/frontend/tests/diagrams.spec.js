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

  // Laid out at the size the renderer asked for, not the natural size its
  // viewBox states, and shown fitted to the measure rather than scrolled in it.
  const emitted = await page.evaluate(() => window.emitted);
  expect(Number.parseFloat(drawn.styleWidth)).toBeCloseTo(emitted.width, 1);
  expect(Number.parseFloat(drawn.styleHeight)).toBeCloseTo(emitted.height, 1);
  expect(drawn.width).toBeCloseTo(drawn.boxWidth, 0);
  expect(drawn.height).toBeGreaterThan(0);
  expect(drawn.scrolls).toBe(false);
  expect(drawn.block).toBeLessThanOrEqual(drawn.container);
  expect(drawn.page).toBeLessThanOrEqual(drawn.viewport);
});

test("output carrying no size of its own is drawn at the renderer's ceiling", async ({ page }) => {
  await page.goto("/tests/diagrams.html");
  const drawn = await page.evaluate(() => (window.drawSizeless(), window.probe()));

  expect(drawn.drawn).toBe(true);
  expect(Number.parseFloat(drawn.styleWidth)).toBeCloseTo(1642 * 0.65, 1);
  expect(Number.parseFloat(drawn.styleHeight)).toBeCloseTo(108 * 0.65, 1);
});

test("a diagram opens whole inside a stage the pane sizes", async ({ page }) => {
  await page.goto("/tests/diagrams.html");
  const emitted = await page.evaluate(() => window.emitted);
  const narrowSize = await page.evaluate(() => window.narrow);
  const page_width = await page.evaluate(() => document.documentElement.scrollWidth);

  const wide = await page.evaluate(() => (window.draw(), window.probe()));
  expect(wide.staged).toBe(true);
  // The <pre> stays unpositioned: the gutter number resolves its column against
  // the document, and a positioned block would land it inside the diagram.
  expect(wide.position).toBe("static");
  expect(wide.gutter).toBe('"3"');
  expect(wide.line).toBe("3");
  expect(wide.width).toBeLessThan(emitted.width);
  expect(wide.width).toBeCloseTo(wide.boxWidth, 0);
  expect(wide.boxHeight).toBeCloseTo((wide.boxWidth / emitted.width) * emitted.height, 0);

  const narrow = await page.evaluate(() => (window.drawNarrow(), window.probe(window.lines.narrow)));
  expect(narrow.staged).toBe(true);
  expect(narrow.width).toBeCloseTo(narrowSize.width, 0);
  expect(narrow.boxHeight).toBeCloseTo(narrowSize.height, 0);
  expect(narrow.slack).toBeGreaterThan(0);

  expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBe(page_width);
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

test("ctrl and the wheel zoom about the pointer, and a double-click goes back", async ({ page }) => {
  await page.goto("/tests/diagrams.html");
  await page.evaluate(() => window.draw());

  const box = await page.evaluate(() => window.stageBox());
  const at = { x: box.x + box.width * 0.65, y: box.y + box.height * 0.4 };
  const fitted = await page.evaluate(() => window.view());
  const under = await page.evaluate(([x, y]) => window.contentAt(x, y), [at.x, at.y]);

  await page.mouse.move(at.x, at.y);
  await page.keyboard.down("Control");
  await page.mouse.wheel(0, -240);
  await page.keyboard.up("Control");
  await page.evaluate(() => window.settled());

  const zoomed = await page.evaluate(() => window.view());
  expect(zoomed.scale).toBeGreaterThan(fitted.scale);
  // The point the pointer was on is still the point the pointer is on.
  const held = await page.evaluate(([x, y]) => window.contentAt(x, y), [at.x, at.y]);
  expect(held.x).toBeCloseTo(under.x, 0);
  expect(held.y).toBeCloseTo(under.y, 0);
  // Paint only: the block is the same size it was before the zoom.
  const staged = await page.evaluate(() => window.probe());
  expect(staged.boxWidth).toBeCloseTo(box.width, 0);
  expect(staged.boxHeight).toBeCloseTo(box.height, 0);

  await page.mouse.dblclick(at.x, at.y);
  await expect.poll(() => page.evaluate(() => window.view().scale)).toBeCloseTo(fitted.scale, 3);
  const back = await page.evaluate(() => window.view());
  expect(back.tx).toBeCloseTo(0, 3);
  expect(back.ty).toBeCloseTo(0, 3);
});

test("an unmodified wheel over a diagram scrolls the document past it", async ({ page }) => {
  await page.goto("/tests/diagrams.html");
  await page.evaluate(() => window.draw());
  const box = await page.evaluate(() => window.stageBox());
  const before = await page.evaluate(() => window.view());

  await page.mouse.move(box.x + box.width / 2, box.y + box.height / 2);
  await page.mouse.wheel(0, 200);
  await page.evaluate(() => window.settled());

  expect(await page.evaluate(() => window.scrolled())).toBeGreaterThan(0);
  expect(await page.evaluate(() => window.view())).toEqual(before);
});

test("zoom stops at eight hundred percent above and at the fitted view below", async ({ page }) => {
  await page.goto("/tests/diagrams.html");
  await page.evaluate(() => window.draw());
  const box = await page.evaluate(() => window.stageBox());
  const fitted = await page.evaluate(() => window.view());

  await page.mouse.move(box.x + box.width / 2, box.y + box.height / 2);
  await page.keyboard.down("Control");
  for (let turn = 0; turn < 20; turn++) await page.mouse.wheel(0, -400);
  await page.evaluate(() => window.settled());
  expect(await page.evaluate(() => window.view().scale)).toBeCloseTo(8, 3);

  for (let turn = 0; turn < 40; turn++) await page.mouse.wheel(0, 400);
  await page.keyboard.up("Control");
  await page.evaluate(() => window.settled());
  expect(await page.evaluate(() => window.view().scale)).toBeCloseTo(fitted.base, 3);
});

test("a zoomed diagram is dragged, and cannot be dragged off its own edges", async ({ page }) => {
  await page.goto("/tests/diagrams.html");
  await page.evaluate(() => window.draw());
  const box = await page.evaluate(() => window.stageBox());
  const at = { x: box.x + box.width / 2, y: box.y + box.height / 2 };
  const resting = await page.evaluate(() => window.probe());

  // Nothing to pan at the fitted default, and nothing promising otherwise.
  expect(await page.evaluate(() => window.cursor())).toBe("auto");
  await page.mouse.move(at.x, at.y);
  await page.mouse.down();
  await page.mouse.move(at.x - 60, at.y - 10, { steps: 4 });
  await page.mouse.up();
  await page.evaluate(() => window.settled());
  expect(await page.evaluate(() => window.view())).toMatchObject({ tx: 0, ty: 0 });

  await page.mouse.move(at.x, at.y);
  await page.keyboard.down("Control");
  await page.mouse.wheel(0, -600);
  await page.keyboard.up("Control");
  await page.evaluate(() => window.settled());
  expect(await page.evaluate(() => window.cursor())).toBe("grab");
  const zoomed = await page.evaluate(() => window.view());

  await page.mouse.down();
  await page.mouse.move(at.x - 40, at.y - 8, { steps: 4 });
  const held = await page.evaluate(() => window.cursor());
  await page.mouse.up();
  await page.evaluate(() => window.settled());
  const panned = await page.evaluate(() => window.view());
  expect(held).toBe("grabbing");
  expect(panned.tx).toBeLessThan(zoomed.tx);
  expect(panned.ty).toBeLessThan(zoomed.ty);
  expect(await page.evaluate(() => window.selected())).toBe("");

  await page.mouse.move(at.x, at.y);
  await page.mouse.down();
  await page.mouse.move(at.x + 4000, at.y + 4000, { steps: 6 });
  await page.mouse.up();
  await page.evaluate(() => window.settled());
  expect(await page.evaluate(() => window.view())).toMatchObject({ tx: 0, ty: 0 });

  await page.mouse.move(at.x, at.y);
  await page.mouse.down();
  await page.mouse.move(at.x - 8000, at.y - 8000, { steps: 6 });
  await page.mouse.up();
  await page.evaluate(() => window.settled());
  const edge = await page.evaluate(() => window.view());
  const emitted = await page.evaluate(() => window.emitted);
  expect(edge.tx).toBeCloseTo(box.width - emitted.width * edge.scale, 0);
  expect(edge.ty).toBeCloseTo(box.height - emitted.height * edge.scale, 0);

  // Paint only, throughout.
  const after = await page.evaluate(() => window.probe());
  expect(after.boxWidth).toBe(resting.boxWidth);
  expect(after.boxHeight).toBe(resting.boxHeight);
});
