import { expect, test } from "@playwright/test";

// mount() keys WebGL off the user agent, so these run as macOS on any host.
test.use({ userAgent: "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko)" });

test.beforeEach(async ({ page }) => {
  await page.goto("/tests/terminal-rendering.html");
  await page.waitForFunction(() => window.fixture);
  const webgl2 = await page.evaluate(() => !!document.createElement("canvas").getContext("webgl2"));
  test.skip(!webgl2, "this browser offers no webgl2 context");
});

// Breaks if nothing handles onContextLoss, leaving the pane blank, or if
// recovery creates a context at once and evicts another terminal's.
test("a lost context falls back to the DOM renderer in place", async ({ page }) => {
  const numbered = Array.from({ length: 30 }, (_, i) => `line ${i + 1}`).join("\r\n");
  await page.evaluate((text) => { fixture.create(); fixture.send(0, text); fixture.done(0); }, numbered);
  await page.waitForFunction(() => fixture.terminals[0].completed === 1);
  const before = await page.evaluate(() => ({ lines: fixture.lines(0), contexts: fixture.contexts, renderer: fixture.renderer(0) }));
  expect(before.renderer.webgl).toBe(true);

  await page.evaluate(() => fixture.lose(0));
  await page.waitForFunction(() => !fixture.renderer(0).webgl, null, { timeout: 6000 });
  expect(await page.evaluate(() => fixture.lines(0))).toEqual(before.lines);
  expect(await page.evaluate(() => fixture.contexts)).toBe(before.contexts);

  await page.evaluate(() => fixture.send(0, "\r\nafter"));
  await page.waitForFunction(() => fixture.rows(0).includes("after"));

  await page.evaluate(() => { fixture.hide(0); fixture.show(0); });
  expect(await page.evaluate(() => fixture.renderer(0))).toEqual({ webgl: true, lost: false });
  expect(await page.evaluate(() => fixture.contexts)).toBe(before.contexts + 1);
});

// Breaks if hide stops releasing the context or show stops creating one, so
// hidden terminals hold contexts WebKit can evict.
test("only the shown terminal holds a WebGL context", async ({ page }) => {
  const count = 20;
  await page.evaluate((count) => {
    for (let id = 0; id < count; id++) {
      fixture.create();
      if (id > 0) fixture.hide(id - 1);
    }
  }, count);

  let shown = count - 1;
  for (let round = 0; round < 3; round++) {
    for (let next = 0; next < count; next++) {
      const state = await page.evaluate(({ shown, next, count }) => {
        const before = fixture.contexts;
        fixture.hide(shown);
        fixture.show(next);
        const renderers = Array.from({ length: count }, (_, id) => fixture.renderer(id));
        return {
          created: fixture.contexts - before,
          webgl: renderers.flatMap((renderer, id) => (renderer.webgl ? [id] : [])),
          lost: renderers[next].lost,
        };
      }, { shown, next, count });
      expect(state).toEqual({ created: 1, webgl: [next], lost: false });
      shown = next;
    }
  }
});
