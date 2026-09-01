import { expect, test } from "@playwright/test";

test.use({ viewport: { width: 1280, height: 800 } });

const token = (page, name) =>
  page.evaluate((value) => {
    const probe = document.createElement("span");
    probe.style.color = value;
    document.body.append(probe);
    const colour = getComputedStyle(probe).color;
    probe.remove();
    return colour;
  }, `var(${name})`);

test.beforeEach(async ({ page }) => {
  await page.goto("/tests/session-rail.html");
  await page.waitForFunction(() => Boolean(window.sessionRail));
});

test("rows name one repository, count the rest, and state their facts at once", async ({ page }) => {
  const selected = page.getByRole("button", {
    name: "Checkout migration · acme/web +1 · Needs you · 2 active · 3 unseen",
  });
  await expect(selected).toHaveAttribute("aria-current", "page");
  await expect(selected).toContainText("acme/web");
  await expect(selected).toContainText("+1");
  await expect(selected).not.toContainText("a-very-long-editing-repository-name");
  await expect(selected).toContainText("Needs you");
  await expect(selected).toContainText("2 active");
  await expect(selected).toContainText("3 unseen");

  await expect(page.getByRole("button", { name: /Session 2 · No editing repositories/ })).toContainText(
    "No editing repositories",
  );
});

test("a session that is not running says how long it has been idle", async ({ page }) => {
  const idle = page.getByRole("button", { name: /^Session 2/ });
  await expect(idle).toContainText("Idle · 1d");
  await expect(idle).not.toContainText("Not running");
});

test("the badge carries the row's state", async ({ page }) => {
  const avatar = (name) => page.getByRole("button", { name }).locator(".avatar");
  const background = (locator) =>
    locator.evaluate((element) => getComputedStyle(element).backgroundColor);

  expect(await background(avatar(/^Checkout migration/))).toBe(await token(page, "--accent-action"));
  expect(await background(avatar(/^Session 2/))).toBe("rgba(0, 0, 0, 0)");
});

test("activity shows an orchestrator, its leads, and subagents behind a count", async ({ page }) => {
  const activity = page.getByRole("region", { name: "Activity" });
  await expect(activity.getByLabel("Orchestrator · Claude · Waiting for you")).toBeVisible();
  const lead = activity.getByLabel("Lead · QRSPI Planning Lead · Active");
  await expect(lead).toBeVisible();

  // A subagent is a detail, not a row, until it is asked for.
  await expect(activity).not.toContainText("Code Reviewer");
  const disclosure = activity.getByRole("button", { name: /subagent/ });
  await expect(disclosure).toContainText("1 subagent · 0 done");
  await disclosure.click();
  await expect(activity).toContainText("Code Reviewer");

  // Depth is an indent step, and no rank draws a rule down its left.
  const indent = (locator) =>
    locator.locator(".dot").evaluate((element) => element.getBoundingClientRect().left);
  expect(await indent(lead)).toBe(
    (await indent(activity.getByLabel("Orchestrator · Claude · Waiting for you"))) + 15,
  );
  expect(await activity.locator(".rank").evaluate((el) => getComputedStyle(el).borderLeftWidth)).toBe(
    "0px",
  );
});

test("waiting is the orchestrator's alone", async ({ page }) => {
  const activity = page.getByRole("region", { name: "Activity" });
  const waiting = await token(page, "--state-waiting");
  const running = await token(page, "--state-running");
  const dot = (label) =>
    activity.getByLabel(label).locator(".dot").evaluate((el) => getComputedStyle(el).backgroundColor);

  expect(await dot("Orchestrator · Claude · Waiting for you")).toBe(waiting);
  expect(await dot("Lead · QRSPI Planning Lead · Active")).toBe(running);
});

test("the rail prints no name it cannot read", async ({ page }) => {
  const activity = page.getByRole("region", { name: "Activity" });
  await expect(activity).not.toContainText("unavailable");
  await expect(activity).toContainText("Test Verifier");

  await page.evaluate(() => window.sessionRail.removeFinished());
  await expect(activity).not.toContainText("Test Verifier");
});

test("the rail headings all read at one rank, centred", async ({ page }) => {
  const faint = await token(page, "--text-faint");
  for (const heading of ["Sessions", "Activity", "This session"]) {
    const caps = page.locator(".caps", { hasText: heading }).first();
    expect(await caps.evaluate((el) => getComputedStyle(el).color)).toBe(faint);
    expect(await caps.evaluate((el) => getComputedStyle(el).textAlign)).toBe("center");
  }
});

test("the rail's first row starts on the pane header's line", async ({ page }) => {
  const band = page.locator(".band");
  const header = await band.evaluate((el) => getComputedStyle(el).height);
  const paneHeader = await page.evaluate(() => {
    const probe = document.createElement("div");
    probe.style.height = "var(--h-pane-header)";
    document.body.append(probe);
    const height = getComputedStyle(probe).height;
    probe.remove();
    return height;
  });
  expect(header).toBe(paneHeader);
  expect(await band.evaluate((el) => getComputedStyle(el).borderBottomWidth)).toBe("0px");

  // Centred in the band, not sat on its floor: AGENT is centred in the pane
  // header beside it, and sitting the label low left a hand's width of nothing
  // above it.
  const centres = await band.evaluate((el) => {
    const caps = el.querySelector(".caps").getBoundingClientRect();
    const outer = el.getBoundingClientRect();
    return { label: caps.top + caps.height / 2 - outer.top, band: outer.height / 2 };
  });
  expect(centres.label).toBeCloseTo(centres.band, 0);
});

test("provider coverage and missing capabilities are textual", async ({ page }) => {
  const activity = page.getByRole("region", { name: "Activity" });
  await page.evaluate(() => window.sessionRail.rootOnly("codex"));
  const row = page.getByRole("button", { name: /Checkout migration .* 1 active · 3 unseen/ });
  const runningPip = row.locator(".glyph.running");
  await expect(runningPip).toHaveText("●");
  expect(await runningPip.evaluate((pip) => getComputedStyle(pip).color)).toBe(
    await token(page, "--state-running"),
  );
  await expect(activity).toContainText("Orchestrator");
  await expect(activity).toContainText("Codex");
  await expect(activity).not.toContainText("Attention unavailable.");
  await expect(activity).not.toContainText("Codex provides root activity only.");

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

test("the anything-new buttons are cubes that sink when pressed", async ({ page }) => {
  for (const name of ["New session", "Add repos"]) {
    const cube = page.getByRole("button", { name });
    const style = await cube.evaluate((el) => {
      const computed = getComputedStyle(el);
      return { shadow: computed.boxShadow, border: computed.borderTopStyle };
    });
    expect(style.shadow).not.toBe("none");
    expect(style.border).toBe("solid");
  }
});

test("sessions take the rail, and activity sizes to what it holds", async ({ page }) => {
  const sessions = page.getByLabel("Sessions");
  const activity = page.locator(".activity-scroll");
  const box = async (locator) =>
    locator.evaluate((element) => ({
      height: element.getBoundingClientRect().height,
      clientHeight: element.clientHeight,
      scrollHeight: element.scrollHeight,
      scrollTop: element.scrollTop,
    }));

  // Sessions is the only greedy child, so it is the list that overflows.
  expect((await box(sessions)).scrollHeight).toBeGreaterThan((await box(sessions)).clientHeight);
  expect((await box(activity)).height).toBeLessThanOrEqual(250);

  await sessions.evaluate((element) => element.scrollTo(0, element.scrollHeight));
  expect((await box(sessions)).scrollTop).toBeGreaterThan(0);
  expect((await box(activity)).scrollTop).toBe(0);

  await activity.evaluate((element) => element.scrollTo(0, element.scrollHeight));
  expect((await box(activity)).scrollTop).toBeGreaterThan(0);
});

test("an empty activity panel occupies nothing", async ({ page }) => {
  const activity = page.locator(".activity-scroll");
  const before = await activity.evaluate((el) => el.getBoundingClientRect().height);
  await page.evaluate(() => window.sessionRail.quiet());
  const after = await activity.evaluate((el) => el.getBoundingClientRect().height);
  expect(after).toBeLessThan(before);
});
