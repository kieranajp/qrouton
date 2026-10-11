import { expect, test } from "@playwright/test";

const mark = (page, row, label) => page.locator(`[data-row="${row}"]`).getByLabel(label);
const square = (locator) =>
  locator.locator(".square").evaluate((el) => {
    const style = getComputedStyle(el);
    return {
      filled: style.backgroundColor !== "rgba(0, 0, 0, 0)",
      ringed: style.outlineStyle !== "none",
      animated: style.animationName !== "none",
    };
  });

test("a mark is hollow or filled, and ringed while its lead runs", async ({ page }) => {
  await page.goto("/tests/stage-marks.html");

  expect(await square(mark(page, "mixed", "R: not written yet"))).toEqual({ filled: false, ringed: false, animated: false });
  expect(await square(mark(page, "mixed", "P: written"))).toEqual({ filled: true, ringed: false, animated: false });
  expect(await square(mark(page, "mixed", "I: not complete yet, lead running"))).toEqual({
    filled: false,
    ringed: true,
    animated: true,
  });
  expect(await square(mark(page, "all", "R: written, lead running"))).toEqual({ filled: true, ringed: true, animated: true });
});

test("reduced motion keeps the ring and stops the pulse", async ({ page }) => {
  await page.emulateMedia({ reducedMotion: "reduce" });
  await page.goto("/tests/stage-marks.html");

  expect(await square(mark(page, "mixed", "I: not complete yet, lead running"))).toEqual({
    filled: false,
    ringed: true,
    animated: false,
  });
});
