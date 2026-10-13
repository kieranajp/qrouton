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
    name: "Checkout migration · acme/web +1 · Unread · 2 active · 3 new docs",
  });
  await expect(selected).toHaveAttribute("aria-current", "page");
  await expect(selected).toContainText("acme/web");
  await expect(selected).toContainText("+1");
  await expect(selected).not.toContainText("a-very-long-editing-repository-name");
  await expect(selected).toContainText("Unread");
  await expect(selected).toContainText("2 active");
  await expect(selected).toContainText("3 new docs");

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
  await page.setViewportSize({ width: 1280, height: 420 });
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

test("the rail headings all read at one rank, centred", async ({ page }) => {
  const faint = await token(page, "--text-faint");
  for (const heading of ["Sessions"]) {
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

  // Centred in the band, not sat on its floor: the tab labels are centred in
  // the pane header beside it, and sitting the label low left a hand's width of nothing
  // above it.
  const centres = await band.evaluate((el) => {
    const caps = el.querySelector(".caps").getBoundingClientRect();
    const outer = el.getBoundingClientRect();
    return { label: caps.top + caps.height / 2 - outer.top, band: outer.height / 2 };
  });
  expect(centres.label).toBeCloseTo(centres.band, 0);
});

test("a summary without a root state falls back to the active count", async ({ page }) => {
  await page.evaluate(() => window.sessionRail.stateless());
  const row = page.getByRole("button", { name: /Checkout migration .* 1 active · 3 new docs/ });
  const runningPip = row.locator('[data-icon="running"]');
  await expect(runningPip).toHaveCount(1);
  expect(await runningPip.evaluate((pip) => getComputedStyle(pip).color)).toBe(
    await token(page, "--state-running"),
  );
});

test("a live lead's stage square replaces the running icon", async ({ page }) => {
  const row = page.getByRole("button", { name: /^Checkout migration · / });
  await page.evaluate(() => window.sessionRail.headline({ state: "working", active: 2, stage: "implement" }));
  const square = row.locator(".facts .agents .square");
  await expect(square).toHaveCount(1);
  await expect(row.locator('[data-icon="running"]')).toHaveCount(0);
  expect(await square.evaluate((el) => getComputedStyle(el).backgroundColor)).toBe(
    await page.evaluate(() => {
      const probe = document.createElement("span");
      probe.style.color = "var(--state-success)";
      document.body.append(probe);
      const colour = getComputedStyle(probe).color;
      probe.remove();
      return colour;
    }),
  );

  await page.evaluate(() => window.sessionRail.headline({ state: "working", active: 2 }));
  await expect(row.locator('[data-icon="running"]')).toHaveCount(1);
  await expect(row.locator(".square")).toHaveCount(0);
});

test("the stage square keeps the running icon's footprint", async ({ page }) => {
  const row = page.getByRole("button", { name: /^Checkout migration · / });
  const measure = () =>
    row.evaluate((el) => {
      const fact = el.querySelector(".facts .agents");
      const mark = fact.querySelector(".stage, [data-icon='running']");
      const box = (node) => {
        const { x, y, width, height } = node.getBoundingClientRect();
        return { x, y, width, height };
      };
      return { row: box(el), fact: box(fact), mark: box(mark) };
    });
  await page.evaluate(() => window.sessionRail.headline({ state: "working", active: 2 }));
  await expect(row.locator('[data-icon="running"]')).toHaveCount(1);
  const icon = await measure();
  await page.evaluate(() => window.sessionRail.headline({ state: "working", active: 2, stage: "plan" }));
  await expect(row.locator(".stage")).toHaveCount(1);
  const square = await measure();
  expect(square).toEqual(icon);
  await page.waitForTimeout(900);
  expect(await measure()).toEqual(icon);
});

test("the rail holds sessions only; the repositories moved to the Overview", async ({ page }) => {
  await expect(page.getByLabel("Selected session details")).toHaveCount(0);
  await expect(page.getByRole("button", { name: "Add repos" })).toHaveCount(0);
});

test("the anything-new buttons are cubes that sink when pressed", async ({ page }) => {
  for (const name of ["New session"]) {
    const cube = page.getByRole("button", { name });
    const style = await cube.evaluate((el) => {
      const computed = getComputedStyle(el);
      return { shadow: computed.boxShadow, border: computed.borderTopStyle };
    });
    expect(style.shadow).not.toBe("none");
    expect(style.border).toBe("solid");
  }
});

test("sessions take the rail", async ({ page }) => {
  const sessions = page.getByLabel("Sessions");
  const box = async (locator) =>
    locator.evaluate((element) => ({
      clientHeight: element.clientHeight,
      scrollHeight: element.scrollHeight,
      scrollTop: element.scrollTop,
    }));

  // Sessions is the only greedy child, so it is the list that overflows.
  expect((await box(sessions)).scrollHeight).toBeGreaterThan((await box(sessions)).clientHeight);
  await sessions.evaluate((element) => element.scrollTo(0, element.scrollHeight));
  expect((await box(sessions)).scrollTop).toBeGreaterThan(0);
});

test("New session stays pinned below the scrolling list", async ({ page }) => {
  const sessions = page.getByLabel("Sessions");
  const create = page.getByRole("button", { name: "New session" });
  const viewport = page.viewportSize();

  await expect(create).toBeInViewport();
  const before = await create.boundingBox();
  const list = await sessions.boundingBox();
  expect(before.y).toBeGreaterThanOrEqual(list.y + list.height);

  await sessions.evaluate((element) => element.scrollTo(0, element.scrollHeight));
  expect(await sessions.evaluate((element) => element.scrollTop)).toBeGreaterThan(0);

  await expect(create).toBeInViewport();
  const after = await create.boundingBox();
  expect(after).toEqual(before);
  expect(after.y + after.height).toBeLessThanOrEqual(viewport.height);
});

test("a row's headline says what its root is doing", async ({ page }) => {
  const row = () => page.getByRole("button", { name: /^Checkout migration/ });
  const headline = (summary) => page.evaluate((s) => window.sessionRail.headline(s), summary);

  await headline({ state: "working", active: 3, tool: "Bash" });
  await expect(row().locator(".fact.agents")).toHaveText(/Working · 2 agents/);
  await expect(row().locator(".fact.tool")).toHaveCount(0);
  await expect(row()).not.toContainText("Bash");

  await headline({ state: "needs_you", reason: "Which branch should I cut from?", active: 1, attention: "unread" });
  const attention = row().locator(".fact.attention");
  await expect(attention).toHaveText(/Needs you/);
  await expect(attention).toHaveAttribute("title", "Which branch should I cut from?");
  expect(await attention.evaluate((el) => getComputedStyle(el).color)).toBe(await token(page, "--state-waiting"));
  await expect(attention.locator("[data-icon]")).toHaveCount(0);

  await headline({ state: "your_turn", active: 1 });
  await expect(row().locator(".fact.agents")).toHaveText(/Your turn/);

  await headline({ state: "idle", active: 1, coverage: "root" });
  await expect(row().locator(".fact.agents")).toHaveText(/Idle/);
});

test("attention and new docs are text facts that wrap whole inside a growing row", async ({ page }) => {
  const row = () => page.getByRole("button", { name: /^Checkout migration/ });
  const headline = (summary) => page.evaluate((s) => window.sessionRail.headline(s), summary);
  await expect.poll(() => page.locator(".rail").evaluate((el) => el.getBoundingClientRect().width)).toBe(200);
  await headline({ state: "needs_you", active: 1, attention: "unread" });
  const facts = row().locator(".facts");
  await expect(facts.locator(".fact")).toHaveText(["Needs you", "3 new docs"]);
  await expect(facts.locator(".fact.attention [data-icon], .fact.unseen [data-icon]")).toHaveCount(0);

  const layout = await facts.evaluate((el) => {
    const line = parseFloat(getComputedStyle(el).fontSize) * 2;
    const item = el.closest(".item").getBoundingClientRect();
    return {
      line,
      left: el.getBoundingClientRect().left,
      item,
      facts: [...el.querySelectorAll(".fact")].map((fact) => {
        const box = fact.getBoundingClientRect();
        return { left: box.left, top: box.top, bottom: box.bottom, height: box.height, clipped: fact.scrollWidth > fact.clientWidth };
      }),
    };
  });
  for (const fact of layout.facts) {
    expect(fact.height).toBeLessThan(layout.line);
    expect(fact.clipped).toBe(false);
    expect(fact.left).toBeCloseTo(layout.left, 0);
    expect(fact.bottom).toBeLessThanOrEqual(layout.item.bottom);
  }
  expect(layout.facts[1].top).toBeGreaterThan(layout.facts[0].top);
});

test("a user-defined sticker draws its own colour and symbol, and a removed one draws none", async ({ page }) => {
  await page.evaluate(() => {
    window.sessionRailBridge.saveStickers([{ id: "a1b2c3d4", colour: "pink", symbol: "heart", label: "Ship it" }]);
    window.sessionRail.stickerChanged("checkout", "a1b2c3d4");
  });
  let sticker = page.getByRole("button", {
    name: "Change sticker for Checkout migration; current sticker: Pink heart — Ship it",
  });
  await expect(sticker.locator("svg")).toHaveAttribute("data-sticker", "heart");
  expect(await sticker.evaluate((button) => getComputedStyle(button).color)).toBe("rgb(245, 189, 230)");

  await page.evaluate(() => window.sessionRailBridge.saveStickers([]));
  sticker = page.getByRole("button", {
    name: "Set sticker for Checkout migration; current sticker: No sticker",
  });
  await expect(sticker.locator("svg")).toHaveAttribute("data-sticker", "none");
});

test("Unload follows Reload and the expanded menu fits near the viewport edge", async ({ page }) => {
  await page.setViewportSize({ width: 1280, height: 420 });
  await page.getByRole("button", { name: /^Checkout migration ·/ }).evaluate((button) => {
    button.dispatchEvent(new MouseEvent("contextmenu", {
      bubbles: true, clientX: 1275, clientY: 415,
    }));
  });
  await expect(page.locator(".menu button.item")).toHaveText([
    "Reload", "Unload", "Reveal in Finder", "Clean up…",
  ]);
  await expect(page.getByRole("button", { name: "Unload", exact: true })).toBeEnabled();
  const box = await page.locator(".menu").boundingBox();
  expect(box.y).toBeGreaterThanOrEqual(8);
  expect(box.y + box.height).toBeLessThanOrEqual(420);
  expect(box.x + box.width).toBeLessThanOrEqual(1280);
});

test("Unload stays disabled for inactive rows and tracks an open menu's current terminal", async ({ page }) => {
  const row = page.getByRole("button", { name: /^Session 2 ·/ });
  await row.click({ button: "right" });
  const unload = page.getByRole("button", { name: "Unload", exact: true });
  await expect(unload).toBeDisabled();
  await unload.evaluate((button) => button.click());
  expect(await page.evaluate(() => window.sessionRailBridge.calls())).toEqual([]);
  await page.evaluate(() => window.sessionRail.load("session-2"));
  await expect(unload).toBeEnabled();
  await page.evaluate(() => window.sessionRail.unloaded("session-2"));
  await expect(unload).toBeDisabled();
  await page.keyboard.press("Escape");
  await expect(page.locator(".menu")).toBeHidden();
  await expect(page.getByRole("button", { name: "Conversation focus" })).toBeFocused();
});

test("unloading a background row sends its slug, dims it and preserves selection and focus", async ({ page }) => {
  await page.evaluate(() => window.sessionRail.load("session-2"));
  const row = page.getByRole("button", { name: /^Session 2 ·/ });
  await row.click({ button: "right" });
  await page.getByRole("button", { name: "Unload", exact: true }).click();
  await expect(page.locator(".menu")).toBeHidden();
  await expect(page.getByRole("button", { name: "Conversation focus" })).toBeFocused();
  await expect(row.locator(".name")).toHaveCSS("color", await token(page, "--text-muted"));
  await expect(row).toContainText("Idle · 1d");
  await expect(page.getByRole("button", { name: /^Checkout migration ·/ })).toHaveAttribute(
    "aria-current", "page",
  );
  expect(await page.evaluate(() => window.sessionRailBridge.calls())).toEqual([
    { method: "unload", slug: "session-2" },
  ]);
});

test("unloading the selected row reveals a loaded fallback, then leaves dimmed resumable rows", async ({ page }) => {
  await page.getByRole("button", { name: /^Session 2 ·/ }).click();
  await page.getByRole("button", { name: /^Checkout migration ·/ }).click();
  const checkout = page.getByRole("button", { name: /^Checkout migration ·/ });
  const second = page.getByRole("button", { name: /^Session 2 ·/ });
  await checkout.click({ button: "right" });
  await page.getByRole("button", { name: "Unload", exact: true }).click();
  await expect(second).toHaveAttribute("aria-current", "page");
  await expect(checkout.locator(".name")).toHaveCSS("color", await token(page, "--text-muted"));
  await second.click({ button: "right" });
  await page.getByRole("button", { name: "Unload", exact: true }).click();
  await expect(page.locator('[aria-current="page"]')).toHaveCount(0);
  await expect(checkout).toBeVisible();
  await expect(second).toBeVisible();
  await expect(page.getByRole("button", { name: "Conversation focus" })).toBeFocused();
  await checkout.click();
  await expect(checkout).toHaveAttribute("aria-current", "page");
  await checkout.click({ button: "right" });
  await expect(page.getByRole("button", { name: "Unload", exact: true })).toBeEnabled();
});

test("an Unload refusal leaves the row loaded and returns focus with row feedback", async ({ page }) => {
  await page.evaluate(() => window.sessionRailBridge.failNextUnload());
  const row = page.getByRole("button", { name: /^Checkout migration ·/ });
  await row.click({ button: "right" });
  await page.getByRole("button", { name: "Unload", exact: true }).click();
  await expect(page.getByRole("button", { name: "Conversation focus" })).toBeFocused();
  await expect(page.getByRole("alert")).toHaveText("Session could not be unloaded");
  await expect(row).toHaveAttribute("aria-current", "page");
  expect(await page.getByRole("alert").evaluate((node) => node.closest(".row")?.querySelector(".name")?.textContent)).toBe("Checkout migration");
  await row.click({ button: "right" });
  await expect(page.getByRole("button", { name: "Unload", exact: true })).toBeEnabled();
});

const sessionButton = (page, name) => page.getByRole("button", { name: new RegExp("^" + name + " ·") });
const railOrder = (page) => page.locator(".session-list .name").allTextContents();
const dragEvent = (locator, type) => locator.evaluate(
  (element, eventType) => element.dispatchEvent(new DragEvent(eventType, {
    bubbles: true, cancelable: true, dataTransfer: new DataTransfer(),
  })),
  type,
);

test("native pointer dragging moves loaded and dimmed sessions without selection or focus changes", async ({ page }) => {
  const focus = page.getByRole("button", { name: "Conversation focus" });
  await focus.focus();
  const checkout = sessionButton(page, "Checkout migration");
  const third = sessionButton(page, "Session 3");
  await checkout.dragTo(third);
  await expect.poll(() => railOrder(page)).toEqual([
    "Session 2", "Session 3", "Checkout migration",
    ...Array.from({ length: 9 }, (_, i) => "Session " + (i + 4)),
  ]);
  await expect(focus).toBeFocused();
  await expect(checkout).toHaveAttribute("aria-current", "page");
  await third.dragTo(sessionButton(page, "Session 2"));
  await expect.poll(() => railOrder(page)).toEqual([
    "Session 3", "Session 2", "Checkout migration",
    ...Array.from({ length: 9 }, (_, i) => "Session " + (i + 4)),
  ]);
  await expect(focus).toBeFocused();
  await expect(checkout).toHaveAttribute("aria-current", "page");
  expect(await page.evaluate(() => window.sessionRailBridge.calls())).toEqual([
    { method: "reorder", slug: "checkout", targetSlug: "session-3" },
    { method: "reorder", slug: "session-3", targetSlug: "session-2" },
  ]);
});

test("reorder waits for committed chrome and remaps shortcuts to displayed positions", async ({ page }) => {
  await page.evaluate(() => window.sessionRailBridge.deferReorder());
  const checkout = sessionButton(page, "Checkout migration");
  const second = sessionButton(page, "Session 2");
  await dragEvent(second, "dragstart");
  await dragEvent(checkout, "dragover");
  await expect(page.locator(".row.over")).toHaveCount(1);
  await expect(page.locator(".row.dragging")).toHaveCount(1);
  await expect(page.locator(".row.dragging")).toHaveCSS("opacity", "0.4");
  await dragEvent(checkout, "drop");
  await expect(page.locator(".row.over, .row.dragging")).toHaveCount(0);
  expect((await railOrder(page)).slice(0, 2)).toEqual(["Checkout migration", "Session 2"]);
  await expect.poll(() => page.evaluate(() => window.sessionRailBridge.calls())).toContainEqual(
    { method: "reorder", slug: "session-2", targetSlug: "checkout" },
  );
  await page.evaluate(() => window.sessionRailBridge.completeReorder());
  await expect.poll(() => railOrder(page)).toEqual([
    "Session 2", "Checkout migration", ...Array.from({ length: 10 }, (_, i) => "Session " + (i + 3)),
  ]);
  await expect(second.locator(".avatar")).toHaveText("⌘1");
  await expect(checkout.locator(".avatar")).toHaveText("⌘2");
  await page.keyboard.press("Meta+1");
  await expect(second).toHaveAttribute("aria-current", "page");
  expect((await page.evaluate(() => window.sessionRailBridge.calls())).at(-1)).toEqual(
    { method: "show", slug: "session-2" },
  );
});

test("self, cancelled, external and sticker drags do not reorder", async ({ page }) => {
  const checkout = sessionButton(page, "Checkout migration");
  const second = sessionButton(page, "Session 2");
  const initial = await railOrder(page);
  await dragEvent(checkout, "dragstart");
  await dragEvent(checkout, "dragover");
  await expect(page.locator(".row.over")).toHaveCount(0);
  await dragEvent(checkout, "drop");
  await dragEvent(checkout, "dragstart");
  await dragEvent(second, "dragover");
  await expect(page.locator(".row.over")).toHaveCount(1);
  await dragEvent(second, "dragleave");
  await expect(page.locator(".row.over")).toHaveCount(0);
  await dragEvent(checkout, "dragend");
  await expect(page.locator(".row.dragging")).toHaveCount(0);
  await dragEvent(second, "dragover");
  await dragEvent(second, "drop");
  const sticker = page.locator(".row").first().locator(".sticker");
  await expect(checkout).toHaveAttribute("draggable", "true");
  expect(await sticker.evaluate((button) => button.draggable)).toBe(false);
  await dragEvent(sticker, "dragstart");
  await dragEvent(second, "dragover");
  await expect(page.locator(".row.over, .row.dragging")).toHaveCount(0);
  await dragEvent(second, "drop");
  expect(await railOrder(page)).toEqual(initial);
  expect(await page.evaluate(() => window.sessionRailBridge.calls())).toEqual([]);
  await sticker.click();
  await expect.poll(() => page.evaluate(() => window.sessionRailBridge.calls())).toEqual([
    { method: "cycle", slug: "checkout" },
  ]);
});

for (const removed of ["checkout", "session-2"]) {
  test("removing " + removed + " during a drag cancels the move", async ({ page }) => {
    const checkout = sessionButton(page, "Checkout migration");
    const second = sessionButton(page, "Session 2");
    await dragEvent(checkout, "dragstart");
    await dragEvent(second, "dragover");
    await expect(page.locator(".row.over")).toHaveCount(1);
    await page.evaluate((slug) => window.sessionRail.remove(slug), removed);
    await expect(page.locator(".row.over, .row.dragging")).toHaveCount(0);
    await dragEvent(sessionButton(page, "Session 3"), "drop");
    expect(await page.evaluate(() => window.sessionRailBridge.calls())).toEqual([]);
  });
}

test("a refused reorder retains the order, selection and focus and reports on the source", async ({ page }) => {
  await page.evaluate(() => window.sessionRailBridge.failNextReorder());
  const focus = page.getByRole("button", { name: "Conversation focus" });
  await focus.focus();
  const initial = await railOrder(page);
  const second = sessionButton(page, "Session 2");
  await second.dragTo(sessionButton(page, "Checkout migration"));
  await expect(page.getByRole("alert")).toHaveText("Session could not be moved");
  await expect(page.getByRole("group", { name: "Session 2 actions" }).getByRole("alert")).toBeVisible();
  expect(await railOrder(page)).toEqual(initial);
  await expect(sessionButton(page, "Checkout migration")).toHaveAttribute("aria-current", "page");
  await expect(focus).toBeFocused();
  await expect(page.locator(".row.over, .row.dragging")).toHaveCount(0);
});

test("session pointer clicks retain focus while keyboard activation stays accessible", async ({ page }) => {
  const focus = page.getByRole("button", { name: "Conversation focus" });
  const second = sessionButton(page, "Session 2");
  await focus.focus();
  await second.click();
  await expect(focus).toBeFocused();
  await expect(second).toHaveAttribute("aria-current", "page");
  const checkout = sessionButton(page, "Checkout migration");
  await checkout.focus();
  await expect(checkout).toBeFocused();
  await page.keyboard.press("Enter");
  await expect(checkout).toHaveAttribute("aria-current", "page");
  await expect(checkout).toBeFocused();
});
