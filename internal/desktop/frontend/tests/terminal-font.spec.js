import { expect, test } from "@playwright/test";

// Both faces are reached through the terminal stack, so a stack that stopped
// naming the family fails here, and so does a bold left behind by the wait.
test("the terminal stack loads both weights of the bundled Nerd Font", async ({ page }) => {
  await page.goto("/tests/terminal-font.html");
  await expect.poll(() => page.evaluate(() => window.terminalFaces)).toBe(true);
});
