import { expect, test } from "@playwright/test";

const open = async (page, query = "") => {
  await page.goto("/tests/overview.html" + query);
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

test("every tier after the first is divided from the one above by a hairline", async ({ page }) => {
  await open(page);
  const edges = await page.locator(".tier").evaluateAll((tiers) =>
    tiers.map((el) => {
      const style = getComputedStyle(el);
      return { top: style.borderTopWidth, pad: style.paddingTop };
    }),
  );
  expect(edges).toHaveLength(3);
  expect(edges[0].top).toBe("0px");
  for (const edge of edges.slice(1)) expect(edge).toEqual({ top: "1px", pad: "12px" });
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

test("a live lead's row sits as far below its top as a finished lead's", async ({ page }) => {
  await open(page);
  const inset = (name) =>
    lead(page, name).evaluate((toggle) => toggle.getBoundingClientRect().top - toggle.closest(".lead").getBoundingClientRect().top);
  expect(await inset("Implementation")).toBe(await inset("Research"));
});

test("each section label shares a baseline with its first row", async ({ page }) => {
  await open(page);
  const baseline = (locator) =>
    locator.evaluate((el) => {
      const probe = document.createElement("span");
      probe.style.cssText = "display:inline-block;width:0;height:0;vertical-align:baseline";
      el.prepend(probe);
      const y = probe.getBoundingClientRect().bottom;
      probe.remove();
      return y;
    });
  const pairs = [
    [".orchestrator .label", ".orchestrator .provider"],
    [".leads .label", ".leads .name"],
    [".session .label", ".session .repo-name"],
  ];
  for (const [label, row] of pairs) {
    const [a, b] = await Promise.all([baseline(page.locator(label).first()), baseline(page.locator(row).first())]);
    expect(Math.abs(a - b)).toBeLessThanOrEqual(1.5);
  }
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

test("a subagent's state is a Mitre icon and a lead's caret a disclosure", async ({ page }) => {
  await open(page);
  await expect(page.locator(".agent.finished .head [data-icon]")).toHaveAttribute("data-icon", "succeeded");
  await expect(page.locator(".agent:not(.finished) .head [data-icon]")).toHaveCount(3);
  for (const icon of await page.locator(".agent:not(.finished) .head [data-icon]").all()) {
    await expect(icon).toHaveAttribute("data-icon", "running");
  }
  await expect(lead(page, "Implementation").locator("..").locator('[data-icon="disclosure-open"]')).toHaveCount(1);
  await expect(lead(page, "Research").locator("..").locator('[data-icon="disclosure-closed"]')).toHaveCount(1);
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

test("add repos sits on the body column's right edge, wide and narrow", async ({ page }) => {
  for (const query of ["", "?narrow"]) {
    await open(page, query);
    const [button, body] = await Promise.all([
      box(page.getByRole("region", { name: "This session" }).getByRole("button", { name: "Add repos" })),
      box(page.locator(".session .body")),
    ]);
    expect(Math.abs(button.right - body.right)).toBeLessThanOrEqual(1);
  }
});

test("the orchestrator's time sits on the body column's right edge", async ({ page }) => {
  await open(page);
  const [time, body] = await Promise.all([box(page.locator(".orchestrator .time")), box(page.locator(".orchestrator .body"))]);
  expect(Math.abs(time.right - body.right)).toBeLessThanOrEqual(1);
});

test("a long root tool target truncates inside the row, left of the time", async ({ page }) => {
  await open(page, "?longdoing");
  const doing = page.locator(".orchestrator .doing");
  await expect(doing).toBeVisible();
  expect(await doing.evaluate((el) => el.scrollWidth > el.clientWidth)).toBe(true);
  expect(await doing.evaluate((el) => getComputedStyle(el).textOverflow)).toBe("ellipsis");
  const [d, time, body] = await Promise.all([box(doing), box(page.locator(".orchestrator .time")), box(page.locator(".orchestrator .body"))]);
  expect(d.right).toBeLessThanOrEqual(time.left);
  expect(time.right).toBeLessThanOrEqual(body.right + 1);
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
