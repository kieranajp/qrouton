import { expect, test } from "@playwright/test";

const open = async (page, query = "") => {
  await page.goto("/tests/agents.html" + query);
  await page.waitForSelector(".head");
};

const indent = (locator) => locator.evaluate((el) => el.getBoundingClientRect().left + parseFloat(getComputedStyle(el).paddingLeft));

test("a grandchild sits deeper than its parent, behind the finished toggle", async ({ page }) => {
  await open(page);

  await expect(page.locator(".head")).toContainText("Claude Code");
  await expect(page.locator(".head")).toContainText("waiting on agents");

  const child = page.getByRole("button", { name: "Spike B · working" });
  await expect(child).toBeVisible();
  await expect(page.getByRole("button", { name: /Spike A/ })).toHaveCount(0);

  await page.getByRole("button", { name: "1 finished" }).click();
  const grandchild = page.getByRole("button", { name: "Spike A · done" });
  await expect(grandchild).toBeVisible();
  expect(await indent(grandchild)).toBeGreaterThan(await indent(child));
  await expect(grandchild).toContainText("42.2k tokens");
});

test("clicking a row expands its tool history and outcome", async ({ page }) => {
  await open(page);
  await page.getByRole("button", { name: "1 finished" }).click();

  const grandchild = page.getByRole("button", { name: "Spike A · done" });
  await expect(grandchild).toHaveAttribute("aria-expanded", "false");
  await expect(page.locator(".event")).toHaveCount(0);
  await grandchild.click();
  await expect(grandchild).toHaveAttribute("aria-expanded", "true");
  await expect(page.locator(".event")).toContainText("Bash");
  await expect(page.locator(".event")).toContainText("echo fg");
  await expect(page.locator(".message")).toHaveText("Done. Output was `fg`.");
});

test("a provider that reports only the root shows its coverage note", async ({ page }) => {
  await open(page, "?tree=opencode");

  await expect(page.locator(".coverage")).toHaveText(
    "OpenCode reports the root agent only, so working and idle come from its terminal.",
  );
  await expect(page.locator(".head")).toContainText("idle");
});

test("a pushed tree replaces the drawn one", async ({ page }) => {
  await open(page);
  await expect(page.getByRole("button", { name: "Spike B · working" })).toBeVisible();

  await page.evaluate(() => window.pushTree("opencode"));
  await expect(page.locator(".coverage")).toBeVisible();
  await expect(page.getByRole("button", { name: /Spike B/ })).toHaveCount(0);
});
