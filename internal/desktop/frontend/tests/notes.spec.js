import { expect, test } from "@playwright/test";

const NEXT = {
  index: 3,
  total: 7,
  title: "Fourth",
  html: '<ul><li>One point</li><li>Another, with <a href="https://example.com/deck">a link</a></li></ul>',
};

const open = async (page, query = "") => {
  await page.goto(`/tests/notes.html${query}`);
  await page.locator(".notes").waitFor();
};

test("the page opens on the note the workbench was already holding", async ({ page }) => {
  await open(page);
  await expect(page.locator(".title")).toHaveText("Third");

  const shown = await page.evaluate(() => window.shown());
  expect(shown.counter).toBe("3 / 7");
  expect(shown.body).toContain("The note the workbench was holding.");
});

test("a step replaces the note, rendered as real markup", async ({ page }) => {
  await open(page);
  await page.evaluate((note) => window.pushNote(note), NEXT);
  await expect(page.locator(".title")).toHaveText("Fourth");

  const shown = await page.evaluate(() => window.shown());
  expect(shown.counter).toBe("4 / 7");
  expect(shown.lists).toBe(2);
  expect(shown.links).toEqual(["https://example.com/deck"]);
  expect(shown.body).not.toContain("The note the workbench was holding.");
});

test("a slide with no notes shows its number and nothing else", async ({ page }) => {
  await open(page);
  await page.evaluate(() => window.pushNote({ index: 4, total: 7, title: "Fifth", html: "" }));
  await expect(page.locator(".title")).toHaveText("Fifth");

  const shown = await page.evaluate(() => window.shown());
  expect(shown.counter).toBe("5 / 7");
  expect(shown.body).toBe("");
});

test("a page the workbench has told nothing carries no counter", async ({ page }) => {
  await open(page, "?blank=1");

  const shown = await page.evaluate(() => window.shown());
  expect(shown.counter).toBe("");
  expect(shown.title).toBe("");
  expect(shown.body).toBe("");
});

test("the band is the only handle the window has", async ({ page }) => {
  await open(page);
  const band = await page.evaluate(() => {
    const style = getComputedStyle(document.querySelector(".band"));
    return {
      drag: style.getPropertyValue("--wails-draggable").trim(),
      padding: Number.parseFloat(style.paddingLeft),
    };
  });

  expect(band.drag).toBe("drag");
  expect(band.padding).toBeGreaterThan(0);
});
