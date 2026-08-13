import { expect, test } from "@playwright/test";

test.beforeEach(async ({ page }) => {
  await page.goto("/tests/diff.html");
  await expect(page.locator(".diff-line")).toHaveCount(16);
});

test("renders fixed old and new gutters with exact hunk coordinates", async ({ page }) => {
  const rows = page.locator(".diff-line");
  await expect(rows.locator(".diff-old")).toHaveCount(16);
  await expect(rows.locator(".diff-new")).toHaveCount(16);

  const coordinates = await rows.evaluateAll((items) => items.map((row) => [
    row.querySelector(".diff-old").dataset.line,
    row.querySelector(".diff-new").dataset.line,
  ]));
  expect(coordinates.slice(6, 15)).toEqual([
    ["", ""],
    ["98", "198"],
    ["99", ""],
    ["", "199"],
    ["100", ""],
    ["", "200"],
    ["101", "201"],
    ["", "202"],
    ["", ""],
  ]);

  const pseudoContent = await rows.nth(7).locator(".diff-gutter").evaluateAll((gutters) =>
    gutters.map((gutter) => getComputedStyle(gutter, "::before").content));
  expect(pseudoContent).toEqual(["\"98\"", "\"198\""]);
});

test("keeps raw markers and classes while non-content rows have blank gutters", async ({ page }) => {
  const blankRows = [0, 1, 2, 3, 4, 5, 6, 14, 15];
  for (const index of blankRows) {
    const gutters = page.locator(".diff-line").nth(index).locator(".diff-gutter");
    await expect(gutters.first()).toHaveAttribute("data-line", "");
    await expect(gutters.last()).toHaveAttribute("data-line", "");
    expect(await gutters.evaluateAll((items) =>
      items.map((gutter) => getComputedStyle(gutter, "::before").content)))
      .toEqual(["\"\"", "\"\""]);
  }

  await expect(page.locator(".diff-line").nth(8)).toHaveClass(/diff-del/);
  await expect(page.locator(".diff-line").nth(8).locator(".diff-content"))
    .toHaveText("--- metadata-looking deletion");
  await expect(page.locator(".diff-line").nth(9)).toHaveClass(/diff-add/);
  await expect(page.locator(".diff-line").nth(9).locator(".diff-content"))
    .toHaveText("+++ metadata-looking addition");
  await expect(page.locator(".diff-line").nth(14)).toHaveClass(/diff-marker/);
  await expect(page.locator(".diff-line").nth(15).locator(".diff-content")).toBeEmpty();
  expect((await page.locator(".diff-line").nth(15).boundingBox()).height).toBeGreaterThan(0);
});

test("wraps long content beneath the content column with aligned fixed gutters", async ({ page }) => {
  await page.evaluate(() => window.setPaneWidth(400));
  const longRow = page.locator(".diff-line").nth(12);
  const geometry = await longRow.evaluate((row) => {
    const oldGutter = row.querySelector(".diff-old");
    const newGutter = row.querySelector(".diff-new");
    const content = row.querySelector(".diff-content");
    const range = document.createRange();
    range.selectNodeContents(content);
    return {
      row: row.getBoundingClientRect().toJSON(),
      oldGutter: oldGutter.getBoundingClientRect().toJSON(),
      newGutter: newGutter.getBoundingClientRect().toJSON(),
      content: content.getBoundingClientRect().toJSON(),
      fragments: [...range.getClientRects()].map((rect) => rect.toJSON()),
    };
  });

  expect(geometry.fragments.length).toBeGreaterThan(1);
  expect(geometry.row.height).toBeGreaterThan(30);
  expect(geometry.oldGutter.top).toBeCloseTo(geometry.content.top, 0);
  expect(geometry.newGutter.top).toBeCloseTo(geometry.content.top, 0);
  expect(geometry.content.left).toBeGreaterThanOrEqual(geometry.newGutter.right);
  for (const fragment of geometry.fragments) {
    expect(fragment.left).toBeGreaterThanOrEqual(geometry.content.left - 1);
  }

  const fixed = await page.locator(".diff-line").evaluateAll((rows) => rows.slice(7, 11).map((row) => {
    const oldGutter = row.querySelector(".diff-old");
    const newGutter = row.querySelector(".diff-new");
    return {
      old: oldGutter.getBoundingClientRect().toJSON(),
      next: newGutter.getBoundingClientRect().toJSON(),
      oldAlign: getComputedStyle(oldGutter).textAlign,
      newAlign: getComputedStyle(newGutter).textAlign,
    };
  }));
  expect(new Set(fixed.map(({ old }) => old.width)).size).toBe(1);
  expect(new Set(fixed.map(({ next }) => next.width)).size).toBe(1);
  expect(new Set(fixed.map(({ old }) => old.right)).size).toBe(1);
  expect(new Set(fixed.map(({ next }) => next.right)).size).toBe(1);
  expect(fixed.every(({ old, next, oldAlign, newAlign }) =>
    old.width === next.width && oldAlign === "right" && newAlign === "right")).toBe(true);
});

test("scrolls narrow panes without overlapping or clipping large coordinates", async ({ page }) => {
  await page.evaluate(async () => {
    await window.setDiff("@@ -9007199254740991 +7000000000000000 @@\n-old\n+new");
    window.setPaneWidth(180);
  });

  const measurements = await page.locator(".diff-grid").evaluate((grid) => {
    const row = grid.querySelectorAll(".diff-line")[1];
    const oldGutter = row.querySelector(".diff-old");
    const newGutter = row.querySelector(".diff-new");
    const content = row.querySelector(".diff-content");
    return {
      clientWidth: grid.clientWidth,
      scrollWidth: grid.scrollWidth,
      old: oldGutter.getBoundingClientRect().toJSON(),
      next: newGutter.getBoundingClientRect().toJSON(),
      content: content.getBoundingClientRect().toJSON(),
      oldClientWidth: oldGutter.clientWidth,
      oldScrollWidth: oldGutter.scrollWidth,
      oldValue: oldGutter.dataset.line,
      oldPseudo: getComputedStyle(oldGutter, "::before").content,
    };
  });

  expect(measurements.scrollWidth).toBeGreaterThan(measurements.clientWidth);
  expect(measurements.oldValue).toBe("9007199254740991");
  expect(measurements.oldPseudo).toBe("\"9007199254740991\"");
  expect(measurements.oldScrollWidth).toBeLessThanOrEqual(measurements.oldClientWidth);
  expect(measurements.old.right).toBeLessThanOrEqual(measurements.next.left + 1);
  expect(measurements.next.right).toBeLessThanOrEqual(measurements.content.left + 1);
});

test("copies and exposes only raw diff content", async ({ page }) => {
  const selected = await page.evaluate(() => window.selectDiff());
  expect(selected).toBe(await page.evaluate(() => window.defaultDiff));
  expect(selected).not.toContain("98 context before");
  expect(selected).not.toContain("198 context before");

  const contextRow = page.locator(".diff-line").nth(7);
  await expect(contextRow.locator(".diff-gutter")).toHaveCount(2);
  await expect(contextRow.locator(".diff-gutter").first()).toHaveAttribute("aria-hidden", "true");
  await expect(contextRow.locator(".diff-gutter").last()).toHaveAttribute("aria-hidden", "true");
  const gutterStyles = await contextRow.locator(".diff-gutter").first().evaluate((gutter) => ({
    pointerEvents: getComputedStyle(gutter).pointerEvents,
    userSelect: getComputedStyle(gutter).userSelect,
  }));
  expect(gutterStyles).toEqual({ pointerEvents: "none", userSelect: "none" });
  const aria = await contextRow.ariaSnapshot();
  expect(aria).toContain("context before");
  expect(aria).not.toContain("98");
  expect(aria).not.toContain("198");

  const separated = "=== one ===\na\n\n=== two ===\nb\n";
  await page.evaluate((text) => window.setDiff(text), separated);
  expect(await page.evaluate(() => window.selectDiff())).toBe(separated);
  await expect(page.locator(".diff-content").nth(2).locator("br"))
    .toHaveAttribute("aria-hidden", "true");
});
