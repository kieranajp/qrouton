import { expect, test } from "@playwright/test";

const columns = (calls, name) => calls.filter(([call]) => call === name).map(([, , cols]) => cols);

test("a scale change regrows an open terminal and tells the PTY its new size", async ({ page }) => {
  await page.goto("/tests/terminal-scale.html");
  await expect.poll(() => page.evaluate(() => window.terminalScale.fontSize())).toBe(13);
  await expect
    .poll(async () => columns(await page.evaluate(() => window.terminalScale.calls()), "pty.Start"))
    .toHaveLength(1);
  const [started] = columns(await page.evaluate(() => window.terminalScale.calls()), "pty.Start");

  await page.evaluate(() => window.terminalScale.announce(150));

  await expect.poll(() => page.evaluate(() => window.terminalScale.fontSize())).toBe(19.5);
  await expect
    .poll(async () => columns(await page.evaluate(() => window.terminalScale.calls()), "pty.Resize").at(-1))
    .toBeLessThan(started);
});
