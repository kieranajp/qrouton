import { expect, test } from "@playwright/test";

const lines = (page) => page.evaluate(() => window.terminalLifecycle.lines());
const starts = (page) => page.evaluate(() => window.terminalLifecycle.starts());

for (const code of [0, 3]) {
  test(`the terminal component shows completed replay exit ${code} delivered during Start`, async ({ page }) => {
    await page.goto(`/tests/terminal-lifecycle.html?completed=${code}`);
    await expect.poll(() => lines(page)).toEqual(["final marker", `[exited with status ${code}]`]);
    expect(await starts(page)).toHaveLength(1);
    await page.evaluate(async () => { window.terminalLifecycle.repeatExit(); await window.terminalLifecycle.flush(); });
    await expect.poll(() => lines(page)).toEqual(["final marker", `[exited with status ${code}]`]);
  });
}

test("the terminal component keeps one live completion through replay and remount attachment", async ({ page }) => {
  await page.goto("/tests/terminal-lifecycle.html");
  await expect.poll(() => starts(page)).toHaveLength(1);
  await page.evaluate(() => window.terminalLifecycle.finish(3));
  await expect.poll(() => lines(page)).toEqual(["final marker", "[exited with status 3]"]);

  for (let i = 0; i < 2; i++) {
    await page.evaluate(async () => { window.terminalLifecycle.replay(); await window.terminalLifecycle.flush(); });
    await expect.poll(() => lines(page)).toEqual(["final marker", "[exited with status 3]"]);
  }
  expect(await starts(page)).toHaveLength(1);

  await page.evaluate(() => window.terminalLifecycle.remount());
  await expect.poll(() => starts(page)).toHaveLength(2);
  await expect.poll(() => lines(page)).toEqual(["final marker", "[exited with status 3]"]);
  expect((await starts(page)).map(([, id]) => id)).toEqual(["t1", "t1"]);
});

test("a terminal start error stays separate from process completion", async ({ page }) => {
  await page.goto("/tests/terminal-lifecycle.html?start-error");
  await expect.poll(() => lines(page)).toEqual(["[could not start: fixture start failed]"]);
  expect(await starts(page)).toHaveLength(1);
});
