import { expect, test } from "@playwright/test";

const open = async (page, query = "") => {
  await page.goto("/tests/agents.html" + query);
  await page.waitForSelector(".orchestrator .words");
};

const lead = (page, name) => page.getByRole("button", { name: new RegExp(`^${name} · `) });
const box = (locator) => locator.evaluate((el) => el.getBoundingClientRect().toJSON());

test("one tier column: orchestrator, leads in stage order, this session", async ({ page }) => {
  await open(page);
  await expect(page.locator(".tier .label")).toHaveText(["Orchestrator", "Leads", "This session"]);
  await expect(page.locator(".orchestrator")).toContainText("Claude Code");
  await expect(page.locator(".orchestrator")).toContainText("waiting on Implementation");
  await expect(page.locator(".leads .name")).toHaveText(["Research", "Plan", "Implementation"]);

  const [label, body] = await Promise.all([box(page.locator(".leads .label")), box(page.locator(".leads .body"))]);
  expect(body.left).toBeGreaterThan(label.right);
});

test("only the live lead opens; finished leads keep their document and count", async ({ page }) => {
  await open(page);
  await expect(lead(page, "Implementation")).toHaveAttribute("aria-expanded", "true");
  await expect(lead(page, "Research")).toHaveAttribute("aria-expanded", "false");
  await expect(page.locator(".lead").first()).toContainText("R3");
  await expect(page.locator(".lead").first()).toContainText("2 subagents");
  await expect(page.getByRole("button", { name: /^Codebase Researcher/ })).toHaveCount(0);

  await lead(page, "Research").click();
  await expect(page.getByRole("button", { name: /^Codebase Researcher/ })).toBeVisible();

  await page.getByRole("button", { name: "Open R3-retry.md" }).click();
  await expect.poll(() => page.evaluate(() => window.opened)).toEqual(["thoughts/shared/research/R3-retry.md"]);
});

test("a lead's square fills once done and rings while live", async ({ page }) => {
  await open(page);
  const square = (name) =>
    lead(page, name).locator(".square").evaluate((el) => {
      const style = getComputedStyle(el);
      return { filled: style.backgroundColor !== "rgba(0, 0, 0, 0)", ringed: style.outlineStyle !== "none" };
    });
  expect(await square("Research")).toEqual({ filled: true, ringed: false });
  expect(await square("Implementation")).toEqual({ filled: false, ringed: true });
});

test("each subagent says what it is doing now and what it wrote", async ({ page }) => {
  await open(page);
  const implementer = page.locator(".agent").filter({ hasText: "Swap PaymentIntent" });
  await expect(implementer).toContainText("now Edit web/checkout/pay.ts");
  await expect(implementer).toContainText("wrote intent.ts, stripe.go");
  await expect(page.locator(".agent").filter({ hasText: "Partial-capture refunds" })).toContainText("thinking");
  const finished = page.locator(".agent.finished");
  await expect(finished).toContainText("Test Writer");
  await expect(finished).toContainText("wrote refund.test.ts");
});

test("clicking a subagent expands its tool history and outcome", async ({ page }) => {
  await open(page);
  const writer = page.getByRole("button", { name: /^Test Writer/ });
  await writer.click();
  await expect(writer).toHaveAttribute("aria-expanded", "true");
  await expect(page.locator(".message")).toHaveText("Wrote the refund tests.");
  await expect(page.locator(".detail")).toContainText("18.4k tokens");

  await page.getByRole("button", { name: /^Implementer · working/ }).first().click();
  await expect(page.locator(".event")).toContainText("Edit web/checkout/intent.ts");
});

test("this session lists the repositories and adds more", async ({ page }) => {
  await open(page);
  const session = page.getByRole("region", { name: "This session" });
  await expect(session).toContainText("acme/web");
  await expect(session).toContainText("2 commits");
  await expect(session).toContainText("+74");
  await expect(session).toContainText("read-only");
  await session.getByRole("button", { name: "Add repos" }).click();
  await expect.poll(() => page.evaluate(() => window.overviewAdded())).toBe(1);
});

test("narrow, each label heads its tier and finished leads drop their count", async ({ page }) => {
  await open(page, "?narrow");
  const [label, body] = await Promise.all([box(page.locator(".leads .label")), box(page.locator(".leads .body"))]);
  expect(body.top).toBeGreaterThanOrEqual(label.bottom);
  await expect(page.locator(".lead").first().locator(".count")).toBeHidden();
});

test("a provider that reports only the root shows its note and says working", async ({ page }) => {
  await open(page, "?tree=opencode");
  await expect(page.locator(".coverage")).toHaveText(
    "OpenCode reports the root agent only, so working and idle come from its terminal.",
  );
  await expect(page.locator(".orchestrator .words")).toHaveText("working");
  await expect(page.locator(".leads")).toHaveCount(0);
});

test("a pushed tree replaces the drawn one", async ({ page }) => {
  await open(page);
  await expect(lead(page, "Implementation")).toBeVisible();
  await page.evaluate(() => window.pushTree("opencode"));
  await expect(page.locator(".coverage")).toBeVisible();
  await expect(page.locator(".leads")).toHaveCount(0);
});

test("reduced motion keeps the live lead's ring steady", async ({ page }) => {
  await page.emulateMedia({ reducedMotion: "reduce" });
  await open(page);
  const animated = await lead(page, "Implementation")
    .locator(".square")
    .evaluate((el) => getComputedStyle(el).animationName);
  expect(animated).toBe("none");
});
