import { expect, test } from "@playwright/test";

const applied = (page) => page.evaluate(() => window.followFixture.applied());

test.beforeEach(async ({ page }) => {
  await page.goto("/tests/follow.html");
  await page.waitForFunction(() => Boolean(window.followFixture));
});

test("an event before the snapshot answers wins", async ({ page }) => {
  await page.evaluate(() => {
    window.followFixture.start();
    window.followFixture.emit({ value: "live" });
    window.followFixture.answer({ ok: true, value: { value: "stale" } });
  });
  await page.evaluate(() => new Promise((resolve) => setTimeout(resolve, 20)));
  expect(await applied(page)).toEqual([{ value: "live" }]);
});

test("a failed snapshot applies nothing known", async ({ page }) => {
  await page.evaluate(() => {
    window.followFixture.start();
    window.followFixture.answer({ ok: false, error: new Error("gone") });
  });
  await expect.poll(() => applied(page)).toEqual([null]);
});

test("accepts filters the events that apply", async ({ page }) => {
  await page.evaluate(() => {
    window.followFixture.start({ only: "mine" });
    window.followFixture.emit({ only: "theirs" });
    window.followFixture.answer({ ok: true, value: { only: "snapshot" } });
  });
  await expect.poll(() => applied(page)).toEqual([{ only: "snapshot" }]);
  await page.evaluate(() => window.followFixture.emit({ only: "mine" }));
  expect(await applied(page)).toEqual([{ only: "snapshot" }, { only: "mine" }]);
});

test("nothing applies after stop", async ({ page }) => {
  await page.evaluate(() => {
    window.followFixture.start();
    window.followFixture.stop();
    window.followFixture.emit({ value: "late" });
    window.followFixture.answer({ ok: true, value: { value: "later" } });
  });
  await page.evaluate(() => new Promise((resolve) => setTimeout(resolve, 20)));
  expect(await applied(page)).toEqual([]);
});

test("settle hears whether an event beat each snapshot", async ({ page }) => {
  await page.evaluate(() => {
    window.followFixture.start({ settle: true });
    window.followFixture.answer({ ok: true, value: "first" });
  });
  await expect.poll(() => applied(page)).toEqual([{ settled: "first", live: false }]);
  await page.evaluate(() => {
    window.followFixture.refresh();
    window.followFixture.emit("event");
    window.followFixture.answer({ ok: true, value: "second" });
  });
  await expect.poll(() => applied(page)).toEqual([
    { settled: "first", live: false },
    "event",
    { settled: "second", live: true },
  ]);
});
