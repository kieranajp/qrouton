import { expect, test } from "@playwright/test";

const metrics = (page) => page.evaluate(() => window.splitterMetrics());
const settleFrames = (page) =>
  page.evaluate(
    () =>
      new Promise((resolve) =>
        requestAnimationFrame(() => requestAnimationFrame(resolve)),
      ),
  );

test.beforeEach(async ({ page }) => {
  await page.goto("/tests/splitter.html");
  await page.waitForFunction(() => Boolean(window.splitterMetrics));
});

test("drag bursts resize once per frame and release commits the exact final width", async ({ page }) => {
  const splitter = page.getByRole("separator", { name: "Resize fixture pane" });
  const box = await splitter.boundingBox();
  const x = box.x + box.width / 2;
  const y = box.y + box.height / 2;
  await page.mouse.move(x, y);
  await page.mouse.down();

  await splitter.evaluate((node, origin) => {
    for (const clientX of [origin - 20, origin - 60, origin - 100]) {
      node.dispatchEvent(new PointerEvent("pointermove", { bubbles: true, pointerId: 1, clientX }));
    }
  }, x);
  await expect.poll(() => metrics(page)).toMatchObject({
    width: 500,
    resizeCalls: 1,
    commitCalls: 0,
    storageWrites: 0,
    stored: null,
  });

  await splitter.evaluate((node, origin) => {
    for (const clientX of [origin - 120, origin - 140]) {
      node.dispatchEvent(new PointerEvent("pointermove", { bubbles: true, pointerId: 1, clientX }));
    }
    node.dispatchEvent(
      new PointerEvent("pointerup", { bubbles: true, pointerId: 1, clientX: origin - 140 }),
    );
  }, x);
  await page.mouse.up();

  await expect.poll(() => metrics(page)).toMatchObject({
    width: 540,
    resizeCalls: 2,
    commitCalls: 1,
    storageWrites: 1,
    stored: "540",
  });
  await expect(page.locator("#pane")).toHaveCSS("width", "540px");
  await settleFrames(page);
  expect(await metrics(page)).toMatchObject({ resizeCalls: 2, commitCalls: 1 });

  await page.reload();
  await page.waitForFunction(() => Boolean(window.splitterMetrics));
  await expect(page.locator("#pane")).toHaveCSS("width", "540px");
});

test("pointer cancellation commits the latest drag width and drops its pending frame", async ({ page }) => {
  const splitter = page.getByRole("separator", { name: "Resize fixture pane" });
  const box = await splitter.boundingBox();
  const x = box.x + box.width / 2;
  const y = box.y + box.height / 2;
  await page.mouse.move(x, y);
  await page.mouse.down();
  await splitter.evaluate((node, origin) => {
    node.dispatchEvent(
      new PointerEvent("pointermove", { bubbles: true, pointerId: 1, clientX: origin - 80 }),
    );
    node.dispatchEvent(
      new PointerEvent("pointercancel", { bubbles: true, pointerId: 1, clientX: origin - 80 }),
    );
  }, x);
  await page.mouse.up();

  await expect.poll(() => metrics(page)).toMatchObject({
    width: 480,
    resizeCalls: 1,
    commitCalls: 1,
    storageWrites: 1,
    stored: "480",
  });
  await settleFrames(page);
  expect(await metrics(page)).toMatchObject({ resizeCalls: 1, commitCalls: 1 });
});

test("keyboard nudges commit their width and reset removes the stored override", async ({ page }) => {
  const splitter = page.getByRole("separator", { name: "Resize fixture pane" });
  await splitter.focus();
  await page.keyboard.press("ArrowLeft");
  await expect.poll(() => metrics(page)).toMatchObject({
    width: 408,
    resizeCalls: 1,
    commitCalls: 1,
    storageWrites: 1,
    stored: "408",
  });

  await page.keyboard.press("Shift+ArrowRight");
  await expect.poll(() => metrics(page)).toMatchObject({
    width: 368,
    resizeCalls: 2,
    commitCalls: 2,
    storageWrites: 2,
    stored: "368",
  });
  await expect(splitter).toHaveAttribute("aria-valuenow", "368");

  await splitter.dispatchEvent("dblclick");
  await expect.poll(() => metrics(page)).toMatchObject({
    width: 400,
    resetCalls: 1,
    storageWrites: 3,
    stored: null,
  });
  await page.reload();
  await page.waitForFunction(() => Boolean(window.splitterMetrics));
  await expect(page.locator("#pane")).toHaveCSS("width", "400px");
});

test("terminal size notifications share one fit per frame", async ({ page }) => {
  await settleFrames(page);
  await page.evaluate(() => window.resetFits());
  await page.evaluate(() => window.sizeBurst());
  await expect.poll(() => metrics(page)).toMatchObject({ fits: 1 });
  await settleFrames(page);
  expect(await metrics(page)).toMatchObject({ fits: 1 });
});

test("terminal size teardown cancels a pending fit and ignores later notifications", async ({ page }) => {
  await settleFrames(page);
  await page.evaluate(() => window.resetFits());
  await page.evaluate(() => window.stopDuringSizeBurst());
  await settleFrames(page);
  expect(await metrics(page)).toMatchObject({ fits: 0 });

  await page.evaluate(() => window.sizeBurst());
  await settleFrames(page);
  expect(await metrics(page)).toMatchObject({ fits: 0 });
});
