import { expect, test } from "@playwright/test";

test.use({ viewport: { width: 1280, height: 800 } });

test.beforeEach(async ({ page }) => {
  await page.goto("/tests/session-rail.html");
  await page.waitForFunction(() => Boolean(window.sessionRail));
});

test("rows expose writable identity and simultaneous status facts", async ({ page }) => {
  const selected = page.getByRole("button", {
    name: "Checkout migration · acme/web · acme/a-very-long-editing-repository-name · Needs you · 2 active · 3 unseen",
  });
  await expect(selected).toHaveAttribute("aria-current", "page");
  await expect(selected).toHaveAttribute(
    "title",
    "Checkout migration · acme/web · acme/a-very-long-editing-repository-name · Needs you · 2 active · 3 unseen",
  );
  await expect(selected).toContainText("acme/web · acme/a-very-long-editing-repository-name");
  await expect(selected).toContainText("Needs you");
  await expect(selected).toContainText("2 active");
  await expect(selected).toContainText("3 unseen");
  await expect(selected).not.toContainText("acme/reference-docs");

  await expect(page.getByRole("button", { name: /Session 2 · No editing repositories/ })).toContainText(
    "No editing repositories",
  );
});

test("selected activity is honest about hierarchy, capabilities, and recent records", async ({ page }) => {
  const activity = page.getByRole("region", { name: "Activity" });
  await expect(activity.getByRole("treeitem", { name: "Orchestrator · Claude · Waiting for you" })).toBeVisible();
  await expect(activity.getByRole("treeitem", { name: "Lead · QRSPI Planning Lead · Active" })).toHaveAttribute(
    "aria-level",
    "2",
  );
  await expect(activity.getByRole("listitem", { name: /Specialist · Code Reviewer · Active · Parent unavailable/ })).toBeVisible();
  await expect(activity).toContainText("Parent relationships and outcomes unavailable.");
  await expect(activity).toContainText("Test Verifier");
  await expect(activity).toContainText("Finished");

  await page.evaluate(() => window.sessionRail.removeFinished());
  await expect(activity).not.toContainText("Test Verifier");
});

test("root-only providers and missing capabilities are textual", async ({ page }) => {
  const activity = page.getByRole("region", { name: "Activity" });
  await page.evaluate(() => window.sessionRail.rootOnly("codex"));
  const row = page.getByRole("button", { name: /Checkout migration .* Root active · 3 unseen/ });
  await expect(row).not.toContainText("Attention unknown");
  const runningPip = row.locator(".glyph.running");
  await expect(runningPip).toHaveText("●");
  expect(await runningPip.evaluate((pip) => getComputedStyle(pip).color)).toBe(
    await page.evaluate(() => {
      const probe = document.createElement("span");
      probe.style.color = "var(--state-running)";
      document.body.append(probe);
      const color = getComputedStyle(probe).color;
      probe.remove();
      return color;
    }),
  );
  await expect(activity).toContainText("Orchestrator");
  await expect(activity).toContainText("Codex");
  await expect(activity).toContainText("Codex provides root activity only.");
  await expect(activity).toContainText("Attention, delegated agents, parent relationships, and outcomes unavailable.");

  await page.evaluate(() => window.sessionRail.rootOnly("opencode"));
  await expect(activity).toContainText("OpenCode provides root activity only.");
});

test("reference detail and Add repos stay in the lower detail stack", async ({ page }) => {
  const detail = page.getByLabel("Selected session details");
  await expect(detail).toContainText("This session");
  await expect(detail).toContainText("acme/reference-docs");
  await expect(detail).toContainText("read-only");
  await detail.getByRole("button", { name: "Add repos" }).click();
  await expect.poll(() => page.evaluate(() => window.sessionRail.added())).toBe(1);
});

test("the session list and selected detail scroll independently", async ({ page }) => {
  const sessions = page.getByLabel("Sessions");
  const detail = page.getByLabel("Selected session details");
  const activity = page.locator(".activity-scroll");
  const dimensions = async (locator) =>
    locator.evaluate((element) => ({
      clientHeight: element.clientHeight,
      scrollHeight: element.scrollHeight,
      scrollTop: element.scrollTop,
    }));

  expect((await dimensions(sessions)).scrollHeight).toBeGreaterThan((await dimensions(sessions)).clientHeight);
  expect((await dimensions(detail)).scrollHeight).toBeGreaterThan((await dimensions(detail)).clientHeight);
  expect((await dimensions(activity)).clientHeight).toBeLessThanOrEqual((await dimensions(detail)).clientHeight * 0.61);
  expect((await dimensions(activity)).scrollHeight).toBeGreaterThan((await dimensions(activity)).clientHeight);
  await sessions.evaluate((element) => element.scrollTo(0, element.scrollHeight));
  expect((await dimensions(sessions)).scrollTop).toBeGreaterThan(0);
  expect((await dimensions(detail)).scrollTop).toBe(0);
  expect((await dimensions(activity)).scrollTop).toBe(0);
  await activity.evaluate((element) => element.scrollTo(0, element.scrollHeight));
  expect((await dimensions(activity)).scrollTop).toBeGreaterThan(0);
  expect((await dimensions(detail)).scrollTop).toBe(0);
  await detail.evaluate((element) => element.scrollTo(0, element.scrollHeight));
  expect((await dimensions(detail)).scrollTop).toBeGreaterThan(0);
});
