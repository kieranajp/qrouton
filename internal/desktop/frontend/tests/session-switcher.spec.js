import { expect, test } from "@playwright/test";
test.use({ userAgent: "Mozilla/5.0 (X11; Linux x86_64)" });
const rows = [
  { slug: "octopus", name: "Octopus", initials: "OC", terminal: "term-1", repos: [{ name: "web" }], summary: { running: true } },
  { slug: "switcher", name: "Session Switcher", initials: "SS", repos: [{ name: "web" }, { name: "backend" }] },
  { slug: "third", name: "Session Switcher", initials: "SS", repos: [] },
];
async function setup(page) {
  await page.goto("/tests/shell.html");
  await page.evaluate((sessions) => { window.shell.snapshot({ identity: "Octopus", mode: "RPI", terminal: "term-1", sessions }); window.shell.controlShow(); }, rows);
  await expect.poll(() => page.evaluate(() => window.shell.started())).toContainEqual(["Term", "term-1"]);
  await page.evaluate(() => { window.shell.windows("dock", [{ id: "dock", kind: "terminal", label: "shell" }, { id: "doc", kind: "document", label: "Research" }]); });
  await expect(page.locator('[data-terminal-id="term-1"] textarea')).toBeAttached();
  await expect.poll(() => page.evaluate(() => window.shell.started())).toContainEqual(["Windows", "dock"]);
}
const query = (page) => page.getByRole("combobox");
for (const mac of [true, false]) {
  test.describe(mac ? "macOS" : "Linux", () => {
    test.use({ userAgent: mac ? "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)" : "Mozilla/5.0 (X11; Linux x86_64)" });
    for (const terminal of ["term-1", "dock"]) {
      test(`chord from ${terminal} owns input, repeats and restores cancellation`, async ({ page }) => {
        await setup(page);
        await page.evaluate((id) => window.shell.focus(id), terminal);
        await page.keyboard.press(mac ? "Control+k" : "Meta+k");
        await expect(query(page)).toHaveCount(0);
        if (mac) await expect.poll(() => page.evaluate(() => window.shell.allWrites().map((entry) => entry[2]))).toContain("\u000b");
        const before = await page.evaluate(() => window.shell.allWrites().length);
        await page.keyboard.down(mac ? "Meta" : "Control");
        await page.keyboard.down("k");
        await page.keyboard.down("k");
        await page.keyboard.up("k");
        await page.keyboard.up(mac ? "Meta" : "Control");
        await expect(query(page)).toBeFocused();
        expect(await page.evaluate(() => window.shell.allWrites().length)).toBe(before);
        await page.keyboard.press("Escape");
        await expect.poll(() => page.evaluate(() => window.shell.focused())).toBe(terminal);
      });
    }
  });
}

test("accessible search, wrapping, live removal, no results, current no-op and tab containment", async ({ page }) => {
  await setup(page);
  await page.keyboard.press("Control+k");
  await expect(page.getByRole("dialog")).toHaveAttribute("aria-modal", "true");
  await expect(query(page)).toBeFocused();
  await expect(page.getByRole("option", { selected: true })).toContainText("switcher");
  await page.keyboard.press("ArrowUp");
  await expect(page.getByRole("option", { selected: true })).toContainText("Octopus");
  await page.keyboard.press("Enter");
  await expect(query(page)).toHaveCount(0);
  expect(await page.evaluate(() => window.shell.shows())).toEqual([]);
  await page.keyboard.press("Control+k");
  await query(page).fill("swch bcknd");
  await expect(page.getByRole("option")).toHaveCount(1);
  await expect(page.getByRole("option")).toContainText("backend");
  const id = await query(page).getAttribute("aria-activedescendant");
  await expect(page.locator(`[id="${id}"]`)).toHaveAttribute("aria-selected", "true");
  await expect(page.getByRole("option").locator("button")).toHaveCount(0);
  await page.keyboard.press("Shift+Tab");
  await expect(page.getByRole("button", { name: "Close session picker" })).toBeFocused();
  await page.keyboard.press("Tab");
  await expect(query(page)).toBeFocused();
  await query(page).fill("absent");
  await expect(page.getByRole("status")).toHaveText("No matching sessions");
  await page.keyboard.press("Enter");
  expect(await page.evaluate(() => window.shell.shows())).toEqual([]);
  await query(page).fill("");
  await page.evaluate((sessions) => window.shell.snapshot({ sessions: sessions.slice(0, 1) }), rows);
  await expect(page.getByRole("option", { selected: true })).toContainText("Octopus");
});

for (const chromeFirst of [true, false]) test(`successful Show with ${chromeFirst ? "chrome" : "reply"} first waits for destination terminal`, async ({ page }) => {
  await setup(page);
  await page.keyboard.press("Control+k");
  await query(page).fill("backend");
  await page.keyboard.press("Enter");
  await page.keyboard.press("Enter");
  await page.keyboard.press("Escape");
  await expect(page.getByRole("status")).toHaveText("Opening…");
  expect(await page.evaluate(() => window.shell.shows())).toEqual([["switcher"]]);
  if (chromeFirst) {
    await page.evaluate(() => window.shell.snapshot({ slug: "switcher", terminal: "" }));
    await expect(query(page)).toBeVisible();
    await page.evaluate(() => window.shell.finishShow());
  } else {
    await page.evaluate(() => window.shell.finishShow());
    await expect(query(page)).toHaveCount(0);
    await page.evaluate(() => window.shell.snapshot({ slug: "switcher", terminal: "" }));
  }
  await page.evaluate(() => window.shell.snapshot({ terminal: "term-2" }));
  await expect.poll(() => page.evaluate(() => window.shell.focused())).toBe("term-2");
});

test("failure retains query, announces and retries; external navigation ignores stale completion", async ({ page }) => {
  await setup(page);
  await page.keyboard.press("Control+k");
  await query(page).fill("backend");
  await page.keyboard.press("Enter");
  await page.evaluate(() => window.shell.finishShow(false));
  await expect(page.getByRole("alert")).toContainText("could not be opened");
  await expect(query(page)).toHaveValue("backend");
  await expect(query(page)).toBeFocused();
  await page.keyboard.press("Enter");
  await page.evaluate(() => window.shell.snapshot({ slug: "third", terminal: "term-3" }));
  await expect(query(page)).toHaveCount(0);
  await page.evaluate(() => window.shell.finishShow());
  await expect(query(page)).toHaveCount(0);
});

test("background chords are blocked, scale works, covering overlay takes ownership", async ({ page }) => {
  await setup(page);
  await page.keyboard.press("Control+k");
  await page.keyboard.press("Meta+2");
  await page.keyboard.press("Control+,");
  await page.keyboard.press("Control+f");
  await expect(query(page)).toBeFocused();
  expect(await page.evaluate(() => window.shell.shows())).toEqual([]);
  await page.evaluate(() => window.shell.snapshot({ placing: true }));
  await expect(query(page)).toHaveCount(0);
  await page.keyboard.press("Control+k");
  await expect(query(page)).toHaveCount(0);
});

test("destination chrome before rejected Show focuses the authoritative fallback", async ({ page }) => {
  await setup(page);
  await page.keyboard.press("Control+k");
  await page.keyboard.press("Enter");
  await page.evaluate(() => window.shell.snapshot({ slug: "switcher", terminal: "term-2" }));
  await expect(query(page)).toBeFocused();
  await page.evaluate(() => window.shell.finishShow(false));
  await expect.poll(() => page.evaluate(() => window.shell.focused())).toBe("term-2");
});

test("overlay retains a failure arriving while covered, but closes an idle failed search", async ({ page }) => {
  await setup(page);
  await page.keyboard.press("Control+k");
  await query(page).fill("backend");
  await page.keyboard.press("Enter");
  await page.evaluate(() => window.shell.snapshot({ placing: true }));
  await expect(query(page)).toHaveCount(0);
  await page.evaluate(() => window.shell.finishShow(false));
  await page.evaluate(() => window.shell.snapshot({ placing: false }));
  await expect(query(page)).toHaveValue("backend");
  await expect(page.getByRole("alert")).toBeVisible();
  await page.evaluate(() => window.shell.snapshot({ placing: true }));
  await page.evaluate(() => window.shell.snapshot({ placing: false }));
  await expect(query(page)).toHaveCount(0);
});

test("document cancellation and find ownership preserve the reader", async ({ page }) => {
  await setup(page);
  await page.evaluate(() => window.shell.acknowledge("doc"));
  const reader = page.locator('.human .body[tabindex="-1"]');
  await reader.focus();
  await page.keyboard.press("Control+k");
  await expect(query(page)).toBeFocused();
  await page.keyboard.press("Meta+f");
  await expect(page.locator('.find')).toHaveCount(0);
  await page.keyboard.press("Escape");
  await expect(reader).toBeFocused();
});

test("rail menus, asynchronous cleanup inspection and confirmation reserve opening", async ({ page }) => {
  await setup(page);
  const row = page.locator('[data-session-slug="octopus"]');
  await row.click({ button: "right" });
  await page.keyboard.press("Control+k");
  await expect(query(page)).toHaveCount(0);
  await page.evaluate(() => window.shell.inspectCleanup());
  await page.getByText("Clean up…", { exact: true }).click();
  await page.keyboard.press("Control+k");
  await expect(query(page)).toHaveCount(0);
  await page.evaluate(() => window.shell.finishCleanup());
  await expect(page.getByRole("dialog")).toBeVisible();
  await page.keyboard.press("Control+k");
  await expect(query(page)).toHaveCount(0);
});

test("composition, text editing, live chrome and pointer activation keep query ownership", async ({ page }) => {
  await setup(page);
  await page.keyboard.press("Control+k");
  await query(page).fill("backend");
  await query(page).dispatchEvent("keydown", { key: "Enter", isComposing: true });
  expect(await page.evaluate(() => window.shell.shows())).toEqual([]);
  await page.keyboard.press("Home");
  await page.keyboard.type("x");
  await expect(query(page)).toHaveValue("xbackend");
  await page.keyboard.press("Backspace");
  await page.keyboard.press("End");
  await page.evaluate(() => window.shell.snapshot({ sessionSummary: "refreshed" }));
  await expect(query(page)).toBeFocused();
  await expect(page.getByRole("option")).toContainText("backend");
  await expect(page.getByRole("listbox")).toHaveAttribute("id", await query(page).getAttribute("aria-controls"));
  await expect(page.getByRole("option", { selected: true })).toHaveCount(1);
  await page.getByRole("option").click();
  expect(await page.evaluate(() => window.shell.shows())).toEqual([["switcher"]]);
});

test("a terminal focus queued before opening cannot take the query", async ({ page }) => {
  await setup(page);
  await page.evaluate(() => {
    window.shell.snapshot({ terminal: "term-queued" });
    window.dispatchEvent(new KeyboardEvent("keydown", { key: "k", ctrlKey: true, bubbles: true }));
  });
  await expect(query(page)).toBeFocused();
  await page.evaluate(() => new Promise((resolve) => requestAnimationFrame(() => requestAnimationFrame(resolve))));
  await expect(query(page)).toBeFocused();
});

test("query context menu is visible above the picker", async ({ page }) => {
  await setup(page);
  await page.keyboard.press("Control+k");
  await query(page).fill("backend");
  await query(page).evaluate((node) => node.select());
  await query(page).click({ button: "right" });
  const copy = page.getByRole("button", { name: "Copy", exact: true });
  await expect(copy).toBeVisible();
  await expect(copy).toBeEnabled();
  const receivesPointer = await copy.evaluate((node) => { const box = node.getBoundingClientRect(); return node.contains(document.elementFromPoint(box.x + box.width / 2, box.y + box.height / 2)); });
  expect(receivesPointer).toBe(true);
});

test("a press in the panel released on the scrim keeps the switcher open, a press and release on the scrim closes it", async ({ page }) => {
  await setup(page);
  await page.keyboard.press("Control+k");
  const box = await query(page).boundingBox();
  await page.mouse.move(box.x + box.width / 2, box.y + box.height / 2);
  await page.mouse.down();
  await page.mouse.move(4, 4);
  await page.mouse.up();
  await expect(query(page)).toBeVisible();
  await page.mouse.move(4, 4);
  await page.mouse.down();
  await page.mouse.up();
  await expect(query(page)).toHaveCount(0);
});
