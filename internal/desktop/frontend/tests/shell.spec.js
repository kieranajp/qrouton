import { expect, test } from "@playwright/test";

const TABS = [
  { id: "window-1", label: "Shell", kind: "terminal" },
  { id: "window-2", label: "Agent", kind: "terminal" },
];

const selected = (page) => page.locator(".tab.selected .label");

async function open(page, selection = "", mode = "RPI") {
  await page.goto("/tests/shell.html");
  await page.evaluate(
    (mode) =>
      window.shell.chrome({
        identity: "Octopus",
        mode,
        terminal: "term-1",
        branch: "fix/octopus-4b2a",
        repos: [{ name: "acme/web", role: "editing", path: "/sessions/octopus/src/web" }],
      }),
    mode,
  );
  await page.evaluate(
    ([selection, tabs]) => window.shell.windows(selection, tabs),
    /** @type {const} */ ([selection, TABS]),
  );
}

// The conversation and a tab are two Go services behind one terminal.
test("the conversation starts on Term and a tab on Windows", async ({ page }) => {
  await open(page, "window-1");
  await expect.poll(() => page.evaluate(() => window.shell.started())).toEqual([
    ["Term", "term-1"],
    ["Windows", "window-1"],
    ["Windows", "window-2"],
  ]);
});

test("an untouched right pane opens at its wider default", async ({ page }) => {
  await open(page, "window-1");
  await expect(page.locator(".human")).toHaveCSS("width", "640px");
});

test("assistant mode replaces RPI marks with a cube escalation action", async ({ page }) => {
  await open(page, "window-1", "ASSISTANT");
  const header = page.locator(".agent .header");
  await expect(header.getByText("Assistant", { exact: true })).toBeVisible();
  await expect(header.getByLabel("Workflow stages")).toHaveCount(0);

  const escalate = header.getByRole("button", { name: "Escalate", exact: true });
  await expect(escalate).toHaveClass(/\bcube\b/);
  await expect(escalate).toHaveCSS("border-top-style", "solid");

  await escalate.click();
  await expect.poll(() => page.evaluate(() => window.shell.escalations())).toEqual([["octopus"]]);
  await page.evaluate(() =>
    window.shell.chrome({
      identity: "Octopus",
      mode: "ASSISTANT",
      terminal: "term-1",
      picker: true,
      pickerKind: "escalate",
      repos: [],
    }),
  );
  await expect(page.getByRole("button", { name: "Escalate →", exact: true })).toBeVisible();
});

test("RPI mode keeps its workflow marks and has no escalation action", async ({ page }) => {
  await open(page, "window-1", "RPI");
  const header = page.locator(".agent .header");
  await expect(header.getByLabel("Workflow stages")).toBeVisible();
  await expect(header.getByText("Assistant", { exact: true })).toHaveCount(0);
  await expect(header.getByRole("button", { name: "Escalate", exact: true })).toHaveCount(0);
});

test("a retained shell replay replaces phantom cells with its prompt", async ({ page }) => {
  await open(page, "window-1");
  await page.evaluate(() => window.shell.terminalData("window-1", "orphaned a"));
  await expect.poll(() => page.evaluate(() => window.shell.terminalText())).toContain("orphaned a");

  await page.evaluate(() =>
    window.shell.terminalData("window-1", "Lifesum/4_Qrouton/tweaks-66d9\r\n❯ ", true),
  );
  await expect.poll(() => page.evaluate(() => window.shell.terminalText())).toContain(
    "Lifesum/4_Qrouton/tweaks-66d9\n❯",
  );
  await expect.poll(() => page.evaluate(() => window.shell.terminalText())).not.toContain("orphaned a");
});

test("an unselected session opens on its leftmost tab", async ({ page }) => {
  await open(page);
  await expect(selected(page)).toHaveText("Shell");
});

test("a click asks Go, and the strip moves when Go answers", async ({ page }) => {
  await open(page);
  await page.getByRole("button", { name: "Agent" }).click();
  await expect.poll(() => page.evaluate(() => window.shell.selects())).toEqual([
    ["octopus", "window-2"],
  ]);
  await expect(selected(page)).toHaveText("Shell");

  await page.evaluate(
    ([tabs]) => window.shell.windows("window-2", tabs),
    /** @type {const} */ ([TABS]),
  );
  await expect(selected(page)).toHaveText("Agent");
});

// A refused Select used to leave the strip claiming a tab Go had not selected,
// with nothing left to reconcile it.
test("a refused selection leaves the strip where Go put it", async ({ page }) => {
  await open(page, "window-1");
  await page.evaluate(() => window.shell.refuseSelect(true));
  await page.getByRole("button", { name: "Agent" }).click();
  await expect.poll(() => page.evaluate(() => window.shell.selects().length)).toBe(1);
  await expect(selected(page)).toHaveText("Shell");
});

test("a selection naming no open tab selects none of them", async ({ page }) => {
  await open(page, "window-1");
  await expect(selected(page)).toHaveText("Shell");

  await page.evaluate(
    ([tabs]) => window.shell.windows("window-9", tabs),
    /** @type {const} */ ([TABS]),
  );
  await expect(selected(page)).toHaveCount(0);
});

// The menu dropped behind the agent pane's header, which carries a stacking
// order of its own: the titlebar has to outrank the panes it sits above.
test("the session name's menu paints over the pane chrome below it", async ({ page }) => {
  await open(page, "window-1");
  await page.getByRole("button", { name: /Octopus/ }).click();

  const menu = page.locator(".menu");
  await expect(menu).toBeVisible();
  await expect(menu).toContainText("fix/octopus-4b2a");
  await expect(menu).toContainText("Copy path to worktree");

  // The menu overhangs the header, and the band they share belongs to the menu.
  expect(await page.evaluate(() => window.overlapOwner())).toBe("menu");
});
