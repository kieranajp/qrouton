import { expect, test } from "@playwright/test";

const menu = (page) => page.locator(".anchor");
const item = (page, label) => menu(page).getByRole("button", { name: label, exact: true });
const labels = (page) => menu(page).getByRole("button").allInnerTexts();
const defaults = (page) => page.evaluate(() => window.defaults);

test.beforeEach(async ({ page }) => {
  await page.goto("/tests/context-menu.html");
  await page.waitForFunction(() => Boolean(window.defaults));
});

// The whole point: the webview's own menu names a browser the user never opened.
test("every right click is the workbench's, never the webview's", async ({ page }) => {
  for (const id of ["#field", "#prose", "#link", "#chrome", "#terminal"]) {
    await page.locator(id).click({ button: "right" });
    await page.keyboard.press("Escape");
  }
  expect(await defaults(page)).toEqual({ prevented: 5, allowed: 0 });
});

test("inert chrome opens no menu, and still draws none from the webview", async ({ page }) => {
  await page.locator("#chrome").click({ button: "right" });
  await expect(menu(page)).toHaveCount(0);
  expect(await defaults(page)).toEqual({ prevented: 1, allowed: 0 });
});

test("a surface with a menu of its own keeps the click", async ({ page }) => {
  await page.locator("#claimed").click({ button: "right" });
  await expect(menu(page)).toHaveCount(0);
  expect(await defaults(page)).toEqual({ prevented: 1, allowed: 0 });
});

test("a field cuts and pastes through the workbench clipboard", async ({ page }) => {
  await page.locator("#field").fill("typed text");
  await page.locator("#field").selectText();
  await page.locator("#field").click({ button: "right" });
  expect(await labels(page)).toEqual(["Cut", "Copy", "Paste", "Select All"]);

  await item(page, "Cut").click();
  await expect(menu(page)).toHaveCount(0);
  await expect(page.locator("#field")).toHaveValue("");
  expect(await page.evaluate(() => window.clipboardText)).toBe("typed text");
  await expect(page.locator("#field")).toBeFocused();

  await page.locator("#field").click({ button: "right" });
  await item(page, "Paste").click();
  await expect(page.locator("#field")).toHaveValue("typed text");
});

test("a field with nothing selected dims the actions that need one", async ({ page }) => {
  await page.locator("#field").fill("typed text");
  await page.locator("#field").click({ button: "right" });
  await expect(item(page, "Cut")).toBeDisabled();
  await expect(item(page, "Copy")).toBeDisabled();
  await expect(item(page, "Paste")).toBeEnabled();
});

test("a terminal copies its selection and pastes into the process", async ({ page }) => {
  await page.locator("#terminal").click({ button: "right" });
  expect(await labels(page)).toEqual(["Copy", "Paste", "Select All"]);
  await expect(item(page, "Copy")).toBeDisabled();

  await item(page, "Select All").click();
  await expect.poll(() => page.evaluate(() => window.terminalSelection().trim())).toBe("output");

  await page.locator("#terminal").click({ button: "right" });
  await item(page, "Copy").click();
  await expect.poll(() => page.evaluate(() => window.clipboardText.trim())).toBe("output");

  await page.evaluate(() => (window.clipboardText = "from the clipboard"));
  await page.locator("#terminal").click({ button: "right" });
  await item(page, "Paste").click();
  await expect
    .poll(() => page.evaluate(() => window.written.join("")))
    .toContain("from the clipboard");
});

test("a link is opened or copied rather than selected", async ({ page }) => {
  await page.locator("#link").click({ button: "right" });
  expect(await labels(page)).toEqual(["Open Link", "Copy Link"]);
  await item(page, "Copy Link").click();
  expect(await page.evaluate(() => window.clipboardText)).toBe("https://example.com/doc");

  await page.locator("#link").click({ button: "right" });
  await item(page, "Open Link").click();
  expect(await page.evaluate(() => window.openedURL)).toBe("https://example.com/doc");
});

test("selected prose offers a copy, and unselected prose offers nothing", async ({ page }) => {
  await page.locator("#prose").click({ button: "right" });
  await expect(menu(page)).toHaveCount(0);

  await page.evaluate(() => {
    window.getSelection().selectAllChildren(document.querySelector("#prose"));
  });
  await page.locator("#prose").click({ button: "right" });
  expect(await labels(page)).toEqual(["Copy"]);
  await item(page, "Copy").click();
  expect(await page.evaluate(() => window.clipboardText)).toBe("Some prose to select.");
});

test("Escape or a press outside dismisses the menu", async ({ page }) => {
  await page.locator("#field").click({ button: "right" });
  await expect(menu(page)).toHaveCount(1);
  await page.keyboard.press("Escape");
  await expect(menu(page)).toHaveCount(0);

  await page.locator("#field").click({ button: "right" });
  await expect(menu(page)).toHaveCount(1);
  // A point well clear of the menu, which sits under the pointer that opened it.
  await page.mouse.click(600, 420);
  await expect(menu(page)).toHaveCount(0);
});
