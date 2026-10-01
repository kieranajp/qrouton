import { expect, test } from "@playwright/test";

const calls = (page) => page.evaluate(() => window.calls);

test("a ready model is one quiet line", async ({ page }) => {
  await page.goto("/tests/ollama.html?state=ready");
  await expect(page.getByText("Semantic search is ready.")).toBeVisible();
  await expect(page.getByRole("button")).toHaveCount(0);
});

test("a missing model offers the download by name, and progress moves the bar", async ({ page }) => {
  await page.goto("/tests/ollama.html?state=missing");
  await expect(page.getByText("all-minilm:22m-l6-v2-fp16")).toBeVisible();
  await page.getByRole("button", { name: "Download model" }).click();
  await expect.poll(() => calls(page)).toContain("Pull");

  await page.evaluate(() =>
    window.emitOllama({ state: "pulling", detail: "pulling abc", completed: 23e6, total: 46e6 }),
  );
  await expect(page.getByRole("progressbar")).toHaveAttribute("aria-valuenow", "50");
  await expect(page.getByText("23.0 MB of 46.0 MB")).toBeVisible();
  await expect(page.getByText("If qrouton restarts, resume it in Settings.")).toBeVisible();

  await page.evaluate(() =>
    window.emitOllama({ state: "pulling", detail: "pulling abc", completed: 46e6, total: 46e6 }),
  );
  await expect(page.getByRole("progressbar")).toHaveAttribute("aria-valuenow", "100");

  await page.getByRole("button", { name: "Cancel" }).click();
  await expect.poll(() => calls(page)).toContain("CancelPull");
});

test("Ollama installed but not running says to start it, with no download link", async ({ page }) => {
  await page.goto("/tests/ollama.html?state=unreachable&installed=1");
  await expect(page.getByText("Ollama is installed but not running.")).toBeVisible();
  await expect(page.getByRole("button", { name: "Get Ollama" })).toHaveCount(0);
  await page.getByRole("button", { name: "Check again" }).click();
  await expect.poll(async () => (await calls(page)).filter((c) => c === "Check").length).toBe(2);
});

test("Ollama not installed says search is keyword-only and links to the download", async ({ page }) => {
  await page.goto("/tests/ollama.html?state=unreachable");
  await expect(page.getByText("search will be keyword-only")).toBeVisible();
  await page.getByRole("button", { name: "Get Ollama" }).click();
  await expect.poll(() => page.evaluate(() => window.openedURL)).toBe("https://ollama.com/download");
  await expect(page.getByRole("button", { name: "Check again" })).toBeVisible();
});

test("a failed download shows the reason and tries again", async ({ page }) => {
  await page.goto("/tests/ollama.html?state=failed&reason=disk%20full");
  await expect(page.getByText("The download failed: disk full")).toBeVisible();
  await page.getByRole("button", { name: "Try again" }).click();
  await expect.poll(() => calls(page)).toContain("Pull");
});
