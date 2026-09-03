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

test("session and sticker actions are sibling native buttons with separate names", async ({ page }) => {
  const session = page.getByRole("button", { name: /^Checkout migration ·/ });
  const sticker = page.locator(".row").first().locator(".sticker");
  await expect(session).toBeVisible();
  await expect(sticker).toBeVisible();
  expect(
    await session.evaluate((button, stickerButton) => ({
      sameParent: button.parentElement === stickerButton.parentElement,
      nested: button.contains(stickerButton) || stickerButton.contains(button),
    }), await sticker.elementHandle()),
  ).toEqual({ sameParent: true, nested: false });
  await expect(sticker).toHaveAttribute(
    "title",
    "Set sticker",
  );
});

test("tooltips stay succinct while accessible names keep long session context", async ({ page }) => {
  const longName = "Checkout migration for the international subscription storefront";
  await page.evaluate((name) => window.sessionRail.rename("checkout", name), longName);
  const sticker = page.locator(".row").first().locator(".sticker");
  await expect(sticker).toHaveAttribute("title", "Set sticker");
  await expect(sticker).toHaveAccessibleName(
    `Set sticker for ${longName}; current sticker: No sticker`,
  );

  await sticker.click();
  await expect(sticker).toHaveAttribute("title", "Important");
  await expect(sticker).toHaveAccessibleName(
    `Change sticker for ${longName}; current sticker: Blue star — Important`,
  );
});

test("sticker activation never selects or shows its session", async ({ page }) => {
  const secondSticker = page.getByRole("button", {
    name: "Set sticker for Session 2; current sticker: No sticker",
  });
  await secondSticker.click();
  await expect(secondSticker).toHaveAttribute("aria-busy", "true");
  await expect(
    page.getByRole("button", {
      name: "Change sticker for Session 2; current sticker: Blue star — Important",
    }),
  ).toBeVisible();
  await expect(page.getByRole("button", { name: /^Checkout migration ·/ })).toHaveAttribute(
    "aria-current",
    "page",
  );
  expect(await page.evaluate(() => window.sessionRailBridge.calls())).toEqual([
    { method: "cycle", slug: "session-2" },
  ]);

  await page.getByRole("button", { name: /^Session 2 ·/ }).click();
  await expect(page.getByRole("button", { name: /^Session 2 ·/ })).toHaveAttribute(
    "aria-current",
    "page",
  );
  expect(await page.evaluate(() => window.sessionRailBridge.calls())).toEqual([
    { method: "cycle", slug: "session-2" },
    { method: "show", slug: "session-2" },
  ]);
});

test("pointer sticker activation preserves conversation focus", async ({ page }) => {
  const focus = page.getByRole("button", { name: "Conversation focus" });
  await focus.focus();
  const sticker = page.locator(".row").first().locator(".sticker");
  await sticker.click();
  await expect(focus).toBeFocused();
});

test("Tab reaches the sticker and Enter and Space each queue one activation", async ({ page }) => {
  const session = page.getByRole("button", { name: /^Checkout migration ·/ });
  await session.focus();
  await page.keyboard.press("Tab");
  const sticker = page.locator(".row").first().locator(".sticker");
  await expect(sticker).toBeFocused();

  await page.keyboard.press("Enter");
  await expect(sticker).toHaveAccessibleName(
    "Change sticker for Checkout migration; current sticker: Blue star — Important",
  );
  await page.keyboard.press("Space");
  await expect(sticker).toHaveAccessibleName(
    "Change sticker for Checkout migration; current sticker: Green bookmark — Read later",
  );
  await expect.poll(() => page.evaluate(() => window.sessionRailBridge.calls().length)).toBe(2);
});

test("the exact cycle renders fixed shapes and colours, including clear", async ({ page }) => {
  let sticker = page.getByRole("button", {
    name: "Set sticker for Checkout migration; current sticker: No sticker",
  });
  const states = [
    ["Blue star — Important", "Important", "--sticker-blue", "star"],
    ["Green bookmark — Read later", "Read later", "--sticker-green", "bookmark"],
    ["Orange question mark — Needs follow-up", "Needs follow-up", "--sticker-orange", "question"],
    ["Red exclamation mark — Has bugs", "Has bugs", "--sticker-red", "exclamation"],
  ];
  for (const [current, meaning, colour, id] of states) {
    await sticker.click();
    sticker = page.getByRole("button", { name: `Change sticker for Checkout migration; current sticker: ${current}` });
    await expect(sticker).toHaveAttribute("title", meaning);
    await expect(sticker.locator("svg")).toHaveAttribute("aria-hidden", "true");
    await expect(sticker.locator("svg")).toHaveAttribute("data-sticker", id);
    await expect(sticker.locator(`svg:has([d])`)).toHaveCount(1);
    expect(await sticker.evaluate((button) => getComputedStyle(button).color)).toBe(
      await token(page, colour),
    );
    expect(id).toBeTruthy();
  }

  await sticker.click();
  sticker = page.getByRole("button", {
    name: "Set sticker for Checkout migration; current sticker: No sticker",
  });
  await expect(sticker.locator("svg")).toHaveAttribute("data-sticker", "none");
  await expect(sticker.locator("path")).toHaveAttribute("fill", "none");
  await expect(sticker.locator("path")).toHaveAttribute("stroke", "currentColor");
  await expect(page.getByRole("status")).toHaveText("No sticker");
});

test("rapid activations stay ordered in one per-session queue", async ({ page }) => {
  await page.evaluate(() => window.sessionRailBridge.setDelay(80));
  const sticker = page.locator(".row").first().locator(".sticker");
  await sticker.click();
  await sticker.click();
  await sticker.click();
  await sticker.click();
  await expect(sticker).toHaveAttribute("aria-busy", "true");
  await expect(
    page.getByRole("button", {
      name: "Change sticker for Checkout migration; current sticker: Red exclamation mark — Has bugs",
    }),
  ).not.toHaveAttribute("aria-busy", "true");
  expect(await page.evaluate(() => window.sessionRailBridge.calls())).toEqual([
    { method: "cycle", slug: "checkout" },
    { method: "cycle", slug: "checkout" },
    { method: "cycle", slug: "checkout" },
    { method: "cycle", slug: "checkout" },
  ]);
});

test("committed feedback is anchored to its sticker and uses current meanings", async ({ page }) => {
  await page.evaluate(() => window.sessionRail.setStickerLabels({ star: "Ship today" }));
  const sticker = page.getByRole("button", {
    name: "Set sticker for Checkout migration; current sticker: No sticker",
  });
  await sticker.click();
  const status = page.getByRole("status");
  await expect(status).toHaveText("Ship today");
  await expect(status).toHaveAttribute("aria-live", "polite");
  await expect(status).toHaveAttribute("aria-atomic", "true");
  expect(await status.evaluate((node) => Boolean(node.closest(".row")?.querySelector(".sticker")))).toBe(true);
  await expect(status).toBeHidden({ timeout: 2500 });
});

test("a saved chrome label replacement updates an open rail without changing its sticker art", async ({ page }) => {
  let sticker = page.getByRole("button", {
    name: "Set sticker for Checkout migration; current sticker: No sticker",
  });
  await sticker.click();
  sticker = page.getByRole("button", {
    name: "Change sticker for Checkout migration; current sticker: Blue star — Important",
  });
  const before = await sticker.evaluate((button) => ({
    colour: getComputedStyle(button).color,
    shape: button.querySelector("svg")?.dataset.sticker,
  }));

  await page.evaluate(() =>
    window.sessionRailBridge.saveStickerLabels({
      star: "Ship today",
      bookmark: "Read after launch",
      question: "Needs an answer",
      exclamation: "Broken here",
    }),
  );

  sticker = page.getByRole("button", {
    name: "Change sticker for Checkout migration; current sticker: Blue star — Ship today",
  });
  await expect(sticker).toBeVisible();
  expect(
    await sticker.evaluate((button) => ({
      colour: getComputedStyle(button).color,
      shape: button.querySelector("svg")?.dataset.sticker,
    })),
  ).toEqual(before);

  await sticker.click();
  await expect(page.getByRole("status")).toHaveText("Read after launch");
});

test("empty and selected stickers stay usable at minimum and default rail widths", async ({ page }) => {
  const row = page.locator(".row").first();
  const sticker = row.locator(".sticker");
  const name = row.locator(".name");

  for (const width of [160, 200]) {
    await page.evaluate((value) =>
      document.documentElement.style.setProperty("--w-rail", `${value}px`),
    width);
    await expect(sticker).toBeVisible();
    const boxes = await row.evaluate((element) => {
      const rowBox = element.getBoundingClientRect();
      const textBox = element.querySelector(".text").getBoundingClientRect();
      const stickerBox = element.querySelector(".sticker").getBoundingClientRect();
      return {
        topGap: stickerBox.top - rowBox.top,
        rightGap: rowBox.right - stickerBox.right,
        textRight: textBox.right,
        stickerLeft: stickerBox.left,
        stickerWidth: stickerBox.width,
        stickerHeight: stickerBox.height,
      };
    });
    expect(boxes.textRight).toBeLessThanOrEqual(boxes.stickerLeft);
    expect(boxes.topGap).toBeLessThanOrEqual(6);
    expect(boxes.rightGap).toBeLessThanOrEqual(4);
    expect(boxes.stickerWidth).toBe(24);
    expect(boxes.stickerHeight).toBe(24);
    expect(
      await name.evaluate((element) => {
        const style = getComputedStyle(element);
        return [style.overflowX, style.textOverflow, style.whiteSpace];
      }),
    ).toEqual(["hidden", "ellipsis", "nowrap"]);
  }

  await expect(sticker.locator("svg")).toHaveAttribute("data-sticker", "none");
  await expect(sticker.locator("path")).toHaveAttribute("fill", "none");
  await sticker.click();
  await expect(sticker).toHaveAccessibleName(
    "Change sticker for Checkout migration; current sticker: Blue star — Important",
  );
  await expect(sticker.locator("svg")).toHaveAttribute("data-sticker", "star");
  await expect(sticker.locator("path")).toHaveAttribute("fill", "currentColor");
  const feedback = page.getByRole("status");
  await expect(feedback).toHaveText("Important");
  await page.evaluate(() => document.documentElement.style.setProperty("--w-rail", "160px"));
  await sticker.click();
  await expect(feedback).toHaveText("Read later");
  const longMeaning = "Needs follow-up with the international release notes";
  await page.evaluate((question) => window.sessionRail.setStickerLabels({ question }), longMeaning);
  await sticker.click();
  await expect(sticker).toHaveAttribute("title", longMeaning);
  await expect(feedback).toHaveText(longMeaning);
  const [railBox, feedbackBox] = await Promise.all([
    page.locator(".rail").boundingBox(),
    feedback.boundingBox(),
  ]);
  expect(feedbackBox.x).toBeGreaterThanOrEqual(railBox.x);
  expect(feedbackBox.x + feedbackBox.width).toBeLessThanOrEqual(railBox.x + railBox.width);
});

test("feedback flips above a fully visible row at the bottom of the scrollport", async ({ page }) => {
  const list = page.getByLabel("Sessions");
  const row = page.locator(".row").nth(8);
  await row.evaluate((element) => {
    const scrollport = element.closest(".session-list");
    const rowBox = element.getBoundingClientRect();
    const scrollBox = scrollport.getBoundingClientRect();
    scrollport.scrollTop += rowBox.bottom - scrollBox.bottom;
  });

  const [listBox, rowBox] = await Promise.all([list.boundingBox(), row.boundingBox()]);
  expect(rowBox.y).toBeGreaterThanOrEqual(listBox.y);
  expect(rowBox.y + rowBox.height).toBeLessThanOrEqual(listBox.y + listBox.height + 1);

  await row.locator(".sticker").click();
  const feedback = row.getByRole("status");
  await expect(feedback).toHaveText("Important");
  const feedbackBox = await feedback.boundingBox();
  expect(feedbackBox.y + feedbackBox.height).toBeLessThanOrEqual(rowBox.y);
  expect(feedbackBox.y).toBeGreaterThanOrEqual(listBox.y);
});

test("an unbroken configured meaning wraps inside 160px feedback", async ({ page }) => {
  const meaning = "NeedsFollowUpBeforeTheInternationalSubscriptionReleaseCanShip";
  await page.evaluate(({ meaning }) => {
    document.documentElement.style.setProperty("--w-rail", "160px");
    window.sessionRail.setStickerLabels({ star: meaning });
  }, { meaning });

  await page.locator(".row").first().locator(".sticker").click();
  const feedback = page.getByRole("status");
  await expect(feedback).toHaveText(meaning);
  const dimensions = await feedback.evaluate((node) => ({
    clientWidth: node.clientWidth,
    scrollWidth: node.scrollWidth,
    clientHeight: node.clientHeight,
    scrollHeight: node.scrollHeight,
  }));
  expect(dimensions.scrollWidth).toBeLessThanOrEqual(dimensions.clientWidth);
  expect(dimensions.scrollHeight).toBeLessThanOrEqual(dimensions.clientHeight);
});

test("a failed mutation keeps the icon and reports an anchored alert", async ({ page }) => {
  await page.evaluate(() => window.sessionRailBridge.failNext());
  const sticker = page.getByRole("button", {
    name: "Set sticker for Checkout migration; current sticker: No sticker",
  });
  await sticker.click();
  const alert = page.getByRole("alert");
  await expect(alert).toHaveText("Sticker could not be changed");
  await expect(sticker).toHaveAccessibleName(
    "Set sticker for Checkout migration; current sticker: No sticker",
  );
  await expect(sticker).not.toHaveAttribute("aria-busy", "true");
  expect(await alert.evaluate((node) => Boolean(node.closest(".row")?.querySelector(".sticker")))).toBe(true);
});

test("the sticker control shares the row context menu", async ({ page }) => {
  const sticker = page.getByRole("button", {
    name: "Set sticker for Checkout migration; current sticker: No sticker",
  });
  await sticker.click({ button: "right" });
  await expect(page.getByRole("button", { name: "Reveal in Finder" })).toBeVisible();
  await expect(page.getByRole("button", { name: "Clean up…" })).toBeVisible();
});

test("the row context menu leads with Reload and calls it for that row", async ({ page }) => {
  await page.getByRole("button", { name: /^Checkout migration ·/ }).click({ button: "right" });
  const reload = page.getByRole("button", { name: "Reload" });
  await expect(reload).toBeVisible();
  await expect(page.locator(".menu button.item").first()).toHaveText("Reload");
  await reload.click();
  await expect
    .poll(() => page.evaluate(() => window.sessionRailBridge.calls()))
    .toContainEqual({ method: "reload", slug: "checkout" });
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
