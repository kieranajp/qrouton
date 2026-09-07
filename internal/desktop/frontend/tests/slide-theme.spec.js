import { expect, test } from "@playwright/test";

const SURFACE_APP = "rgb(1, 0, 0)";
const SURFACE_CHROME = "rgb(2, 0, 0)";
const SURFACE_TERMINAL = "rgb(3, 0, 0)";
const SURFACE_RAISED = "rgb(4, 0, 0)";
const TEXT_PRIMARY = "rgb(5, 0, 0)";
const TEXT_SECONDARY = "rgb(6, 0, 0)";
const TEXT_MUTED = "rgb(7, 0, 0)";
const TEXT_ON_ACCENT = "rgb(9, 0, 0)";
const ACCENT_LABEL = "rgb(11, 0, 0)";
const ACCENT_LITERAL = "rgb(12, 0, 0)";
const STATE_SUCCESS = "rgb(13, 0, 0)";
const STATE_WAITING = "rgb(14, 0, 0)";
const BORDER_SUBTLE = "rgb(15, 0, 0)";
const BORDER_DEFAULT = "rgb(16, 0, 0)";

const style = (page, selector) => page.evaluate((it) => window.slideStyle(it), selector);

test.beforeEach(async ({ page }) => {
  await page.goto("/tests/slide-theme.html");
});

test("a slide resolves the app's own custom properties", async ({ page }) => {
  const slide = await style(page, "div.marpit > section");
  const heading = await style(page, "div.marpit > section h1");

  expect(slide.background).toBe(SURFACE_APP);
  expect(slide.color).toBe(TEXT_SECONDARY);
  expect(slide.family).toContain("JetBrains Mono Fixture");
  expect(heading.color).toBe(TEXT_PRIMARY);
  expect(heading.family).toContain("Space Grotesk Fixture");
});

test("each layout carries its own ground", async ({ page }) => {
  expect((await style(page, "section:not([class])")).background).toBe(SURFACE_APP);
  expect((await style(page, "section.title")).background).toBe(SURFACE_TERMINAL);
  expect((await style(page, "section.statement")).background).toBe(SURFACE_TERMINAL);
  expect((await style(page, "section.alt")).background).toBe(SURFACE_CHROME);

  expect((await style(page, "section.title h1")).color).toBe(TEXT_PRIMARY);
  expect((await style(page, "section.statement h1")).color).toBe(ACCENT_LABEL);
});

test("each component carries its own border or ground", async ({ page }) => {
  expect((await style(page, ".shot img")).border).toBe(BORDER_SUBTLE);
  expect((await style(page, ".shot figcaption")).color).toBe(TEXT_MUTED);

  expect((await style(page, ".cards > :not(.accent)")).background).toBe(SURFACE_RAISED);
  expect((await style(page, ".cards > .accent")).background).toBe(ACCENT_LABEL);
  expect((await style(page, ".cards > .accent")).color).toBe(TEXT_ON_ACCENT);

  expect((await style(page, ".callout:not(.note):not(.good):not(.warn)")).edge).toBe(BORDER_DEFAULT);
  expect((await style(page, ".callout.note")).edge).toBe(ACCENT_LITERAL);
  expect((await style(page, ".callout.good")).edge).toBe(STATE_SUCCESS);
  expect((await style(page, ".callout.warn")).edge).toBe(STATE_WAITING);

  const note = await style(page, ".note:not(.callout)");
  const callout = await style(page, ".callout.note");
  expect(note.color).toBe(TEXT_MUTED);
  // The callout keeps its own ground and body size when it takes the note tone.
  expect(callout.background).toBe(SURFACE_CHROME);
  expect(callout.size).toBeGreaterThan(note.size);
});

test("cols divides the slide and wide-left unbalances it", async ({ page }) => {
  const [left, right] = await page.evaluate(() => window.slideBoxes(".cols > *"));

  expect(left.width).toBeGreaterThan(right.width * 1.5);
  expect(right.left).toBeGreaterThan(left.left);
});

test("an invented class draws as an unstyled block", async ({ page }) => {
  const invented = await page.evaluate(() => {
    const slide = document.querySelector("div.marpit > section:last-child");
    slide.insertAdjacentHTML("beforeend", '<div class="ledger">not in the vocabulary</div>');
    return window.slideStyle(".ledger");
  });

  expect(invented.background).toBe("rgba(0, 0, 0, 0)");
  expect(invented.border).toBe(TEXT_SECONDARY);
  expect(invented.color).toBe(TEXT_SECONDARY);
});
