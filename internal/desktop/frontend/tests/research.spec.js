import { expect, test } from "@playwright/test";

const LOADER = "How does the loader stamp a skill?";
const KIND = "Where does the kind come from?";
const OPEN = "Open Questions";

const open = async (page, query = "") => {
  await page.goto("/tests/research.html" + query);
  await page.waitForSelector(".item, .markdown", { state: "attached" });
};

const opened = (page) => page.evaluate(() => window.opened());
const marked = (page) => page.evaluate(() => window.markedLines());
const row = (page, name) => page.locator(`.item[data-item="${name}"]`);

test("the summary is pinned open and every question starts closed", async ({ page }) => {
  await open(page);

  await expect(page.locator(".pinned")).toContainText("What it all came to");
  expect(await page.evaluate(() => window.items())).toEqual([
    { name: LOADER, open: false },
    { name: KIND, open: false },
    { name: OPEN, open: false },
  ]);
  // The summary is not one of the rows, so it is not one of the count either.
  expect(await page.evaluate(() => window.counter())).toBe("3 sections");
  await expect(page.locator(".items")).not.toContainText("Summary");
});

test("clicking a question reveals its body and leaves the others closed", async ({ page }) => {
  await open(page);
  await expect.poll(() => page.evaluate(() => window.drawnItems())).toEqual([]);

  await row(page, KIND).locator("summary").click();
  await expect.poll(() => opened(page)).toEqual([KIND]);
  await expect(row(page, KIND)).toContainText("From the path segment");
  await expect.poll(() => page.evaluate(() => window.drawnItems())).toEqual([KIND]);

  // Several may be open at once: this is a document, not a wizard.
  await row(page, LOADER).locator("summary").click();
  await expect.poll(() => opened(page)).toEqual([LOADER, KIND]);
});

// The pane took the open state off the element, so the keyboard path no longer
// rides on the element's own behaviour and has to be held down by a test.
test("Enter and Space on a question work it the same as a click", async ({ page }) => {
  await open(page);
  const summary = row(page, KIND).locator("summary");

  await summary.focus();
  await page.keyboard.press("Enter");
  await expect.poll(() => opened(page)).toEqual([KIND]);

  await page.keyboard.press("Enter");
  await expect.poll(() => opened(page)).toEqual([]);

  await page.keyboard.press("Space");
  await expect.poll(() => opened(page)).toEqual([KIND]);
});

test("a document opened at a line inside a question opens that question", async ({ page }) => {
  await open(page, "?line=15&to=15");

  await expect.poll(() => opened(page)).toEqual([LOADER]);
  await expect.poll(() => marked(page)).toEqual([15]);

  await expect
    .poll(() => page.evaluate(() => window.reports.at(-1)))
    .toMatchObject({ available: true, selected: true });
});

test("a span crossing into the next question marks only the part inside its own", async ({
  page,
}) => {
  await open(page, "?line=15&to=23");
  await expect.poll(() => opened(page)).toEqual([LOADER]);
  await expect.poll(() => marked(page)).toEqual([15, 17]);
});

test("a span in the preamble opens nothing and marks the lead", async ({ page }) => {
  await open(page, "?line=7");
  expect(await opened(page)).toEqual([]);
  await expect.poll(() => marked(page)).toEqual([7]);
});

test("Expand all opens every question and Collapse all shuts them", async ({ page }) => {
  await open(page);

  await page.getByRole("button", { name: "Expand all" }).click();
  await expect.poll(() => opened(page)).toEqual([LOADER, KIND, OPEN]);
  await expect.poll(() => page.evaluate(() => window.drawnItems())).toEqual([LOADER, KIND, OPEN]);

  await page.getByRole("button", { name: "Collapse all" }).click();
  await expect.poll(() => opened(page)).toEqual([]);
});

test("a fenced block and a table inside a question render as themselves", async ({ page }) => {
  await open(page);
  await page.getByRole("button", { name: "Expand all" }).click();

  await expect(row(page, LOADER).locator("pre")).toContainText("func main()");
  await expect(row(page, KIND).locator("table tbody tr")).toHaveCount(1);
  await expect(row(page, KIND).locator("table")).toContainText("right");
});

test("the document view renders the whole research and comes back to the accordion", async ({
  page,
}) => {
  await open(page);
  await row(page, KIND).locator("summary").click();
  await expect.poll(() => opened(page)).toEqual([KIND]);

  await page.getByRole("button", { name: "Document", exact: true }).click();
  await expect.poll(() => page.evaluate(() => window.mode())).toBe("Document");
  await expect(page.locator(".reading h2").first()).toHaveText("Summary");
  await expect(page.locator(".reading")).toContainText("Nothing outstanding.");
  await expect(page.locator(".reading")).not.toContainText("## Open Questions");
  await expect(page.locator(".reading pre")).toContainText("func main()");
  // Reading straight through is the point of the mode; there is nothing to fold.
  await expect(page.locator(".item")).toHaveCount(0);

  await page.getByRole("button", { name: "Research", exact: true }).click();
  await expect.poll(() => page.evaluate(() => window.mode())).toBe("Research");
  await expect.poll(() => opened(page)).toEqual([KIND]);
});

test("the research is named in both views and never twice at once", async ({ page }) => {
  await open(page);
  await expect(page.locator(".head")).not.toContainText("The fixture research");
  await expect(page.locator("h1", { hasText: "The fixture research" })).toHaveCount(1);

  await page.getByRole("button", { name: "Document", exact: true }).click();
  await expect(page.locator(".reading h1").first()).toHaveText("The fixture research");
  await expect(page.locator("h1", { hasText: "The fixture research" })).toHaveCount(1);
});

test("a document with no second-level heading renders as plain markdown", async ({ page }) => {
  await open(page, "?plain=true");
  await expect(page.locator(".item")).toHaveCount(0);
  await expect(page.locator(".markdown")).toContainText("No headings open anything here.");
  expect(await page.evaluate(() => window.errors)).toEqual([]);
});

// A questions brief is this document before anyone answered it, so it opens the
// same way: the summary pinned, a row per question holding its framing.
test("an unanswered document opens as an accordion of its questions", async ({ page }) => {
  await open(page, "?unanswered=true");
  await expect(page.locator(".pinned")).toContainText("What is being looked at");
  expect(await page.evaluate(() => window.items())).toEqual([
    { name: LOADER, open: false },
    { name: KIND, open: false },
    { name: OPEN, open: false },
  ]);
  expect(await page.evaluate(() => window.counter())).toBe("3 sections");

  await row(page, LOADER).locator("summary").click();
  await expect(row(page, LOADER).locator("blockquote")).toContainText("prompts/loader.go");
});

test("a pushed document redraws without shutting the open questions", async ({ page }) => {
  await open(page);
  await row(page, LOADER).locator("summary").click();
  await expect.poll(() => opened(page)).toEqual([LOADER]);

  await page.evaluate(() => window.pushEdited());
  await expect(row(page, LOADER)).toContainText("It now walks the folder");
  await expect.poll(() => opened(page)).toEqual([LOADER]);
  expect(await page.evaluate(() => window.errors)).toEqual([]);
});

test("nothing in the chrome meters the research", async ({ page }) => {
  await open(page);
  await expect(page.locator(".footer")).toBeVisible();
  await expect(page.locator(".bar")).toHaveCount(0);
  await expect(page.locator(".pip")).toHaveCount(0);
  await expect(page.locator(".dot")).toHaveCount(0);
});
