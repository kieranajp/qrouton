import { expect, test } from "@playwright/test";

const RUNNING = "rgb(139, 213, 202)";
const ACTION = "rgb(138, 173, 244)";

const open = async (page, query = "") => {
  await page.goto("/tests/plan.html" + query);
  await page.waitForSelector("[data-screen], .markdown", { state: "attached" });
};

const shown = (page) => page.evaluate(() => window.shown());
const follow = (page) => page.getByRole("checkbox", { name: "Follow" });
const marked = (page) => page.evaluate(() => window.markedLines());

test("a span in a later phase opens that phase and marks only its blocks", async ({ page }) => {
  await open(page, "?line=19&to=21");

  await expect.poll(() => shown(page)).toEqual(["2"]);
  await expect.poll(() => marked(page)).toEqual([19, 21]);

  await expect
    .poll(() => page.evaluate(() => window.reports.at(-1)))
    .toMatchObject({ available: true, selected: true });
  const intervals = await page.evaluate(() => window.reports.at(-1).intervals);
  expect(intervals.length).toBeGreaterThan(0);
  for (const interval of intervals) {
    expect(interval.line).toBeGreaterThanOrEqual(17);
    expect(interval.to).toBeLessThanOrEqual(38);
  }

  await page.keyboard.press("ArrowRight");
  await expect.poll(() => shown(page)).toEqual(["3"]);
  expect(await marked(page)).toEqual([]);
});

test("a span in the preamble opens the overview", async ({ page }) => {
  await open(page, "?line=7");
  await expect.poll(() => shown(page)).toEqual(["overview"]);
  await expect.poll(() => marked(page)).toEqual([7]);
});

test("a span crossing a boundary marks only the part inside its opening phase", async ({ page }) => {
  await open(page, "?line=36&to=41");
  await expect.poll(() => shown(page)).toEqual(["2"]);
  await expect.poll(() => marked(page)).toEqual([36, 37]);
});

test("the overview lists every phase with the count its criteria state", async ({ page }) => {
  await open(page);
  await expect.poll(() => shown(page)).toEqual(["overview"]);
  expect(await page.evaluate(() => window.counters())).toEqual([
    "1 Groundwork 2/2",
    "2 The middle 1/2",
    "3 The end 0/1",
  ]);
});

test("observations render in the body and stay out of the meter", async ({ page }) => {
  await open(page, "?line=19");
  const screen = page.locator('[data-screen="2"]');

  await expect(screen.locator(".markdown:not(.lifted)").first()).toContainText("the deck looks right");
  await expect(screen.locator(".markdown:not(.lifted)").first()).toContainText("the pips line up");
  await expect(screen.locator('[data-count="2"]')).toHaveText("1 of 2 met");
  await expect(screen.locator(".state")).toContainText("Working");
  await expect(screen.locator(".state .dot")).toHaveCSS("background-color", RUNNING);
});

test("arrow keys move between screens and only the shown one is drawn", async ({ page }) => {
  await open(page);
  await expect.poll(() => page.evaluate(() => window.drawnScreens())).toEqual(["overview"]);

  await page.keyboard.press("ArrowRight");
  await expect.poll(() => shown(page)).toEqual(["1"]);
  await expect.poll(() => page.evaluate(() => window.drawnScreens())).toEqual(["1"]);

  await page.keyboard.press("ArrowLeft");
  await expect.poll(() => shown(page)).toEqual(["overview"]);
});

test("a fenced block and a table inside a phase body render as themselves", async ({ page }) => {
  await open(page, "?line=19");
  const screen = page.locator('[data-screen="2"]');

  await expect(screen.locator("pre")).toHaveCount(1);
  await expect(screen.locator("pre")).toContainText("func main()");
  await expect(screen.locator("table tbody tr")).toHaveCount(1);
  await expect(screen.locator("table")).toContainText("right");
});

test("a document nothing opens a phase in renders the plain markdown body", async ({ page }) => {
  await open(page, "?plain=true");
  await expect(page.locator("[data-screen]")).toHaveCount(0);
  await expect(page.locator(".markdown")).toContainText("No headings open anything here.");
});

test("a pip selects its phase and takes the underline", async ({ page }) => {
  await open(page);
  const pip = page.locator('.pip[aria-label="Phase 3"]');

  await pip.click();
  await expect.poll(() => shown(page)).toEqual(["3"]);
  await expect(pip).toHaveAttribute("aria-current", "true");
  await expect(pip).toHaveCSS("border-bottom-color", "rgb(138, 173, 244)");
  await expect(page.locator('.pip[aria-label="Phase 1"]')).toHaveCSS(
    "border-bottom-color",
    "rgba(0, 0, 0, 0)",
  );
});

test("the counter names the overview and the arrows stop at both ends", async ({ page }) => {
  await open(page);
  const previous = page.locator('button[aria-label="Previous screen"]');
  const next = page.locator('button[aria-label="Next screen"]');

  await expect(page.locator(".counter")).toHaveText("Overview");
  await expect(previous).toBeDisabled();
  await expect(next).toBeEnabled();

  await next.click();
  await expect(page.locator(".counter")).toHaveText("1 / 3");
  await next.click();
  await next.click();
  await expect(page.locator(".counter")).toHaveText("3 / 3");
  await expect(next).toBeDisabled();
  await expect(previous).toBeEnabled();
});

test("the document view renders the whole plan and comes back to the same phase", async ({ page }) => {
  await open(page, "?line=19");
  await expect.poll(() => shown(page)).toEqual(["2"]);

  await page.getByRole("button", { name: "Document", exact: true }).click();
  await expect.poll(() => page.evaluate(() => window.mode())).toBe("Document");
  // The whole plan as rendered markdown: headings drawn, source not quoted.
  await expect(page.locator(".reading h2").first()).toHaveText("Phase 1 — Groundwork");
  await expect(page.locator(".reading")).not.toContainText("## Phase 2");
  await expect(page.locator(".reading pre")).toContainText("func main()");
  await expect(page.locator(".reading input[type=checkbox]").first()).toBeChecked();

  await page.getByRole("button", { name: "Plan", exact: true }).click();
  await expect.poll(() => page.evaluate(() => window.mode())).toBe("Plan");
  await expect.poll(() => shown(page)).toEqual(["2"]);
});

// The header stopped naming the plan, so whichever view is open has to.
test("the plan is named in both views and never twice at once", async ({ page }) => {
  await open(page);
  await expect(page.locator(".head")).not.toContainText("The fixture plan");
  await expect(page.locator("h1", { hasText: "The fixture plan" })).toHaveCount(1);
  await expect(page.locator('[data-screen="overview"] h1')).toHaveText("The fixture plan");

  await page.getByRole("button", { name: "Document", exact: true }).click();
  await expect(page.locator(".reading h1").first()).toHaveText("The fixture plan");
  await expect(page.locator("h1", { hasText: "The fixture plan" })).toHaveCount(1);
});

// A section slide puts its name where a phase puts 3 / 6. The name is the one
// label long enough to want a second line, and the footer cannot give it one.
test("a long section name truncates rather than growing the footer", async ({ page }) => {
  const SECTION = "Verification strategy and rollout notes";
  for (const width of [900, 400]) {
    await page.setViewportSize({ width, height: 700 });
    await open(page, "?long=true");

    await page.locator('.pip[aria-label="Phase 3"]').click();
    await expect.poll(() => shown(page)).toEqual(["3"]);
    const phase = await page.evaluate(() => window.footerShape());

    await page.locator(`.pip[aria-label="${SECTION}"]`).click();
    await expect.poll(() => shown(page)).toEqual([SECTION]);
    const section = await page.evaluate(() => window.footerShape());

    expect(section.height, `footer height at ${width}`).toBe(phase.height);
    expect(section.counterHeight, `label height at ${width}`).toBe(phase.counterHeight);
    // The strip says where in the deck the reader is; a second row of pips would
    // misstate the shape of the document.
    expect(phase.pipRows, `pip rows at ${width}`).toBe(1);
    expect(section.pipRows, `pip rows at ${width}`).toBe(1);
    expect(section.counterTitle).toBe(SECTION);
  }
  // Narrowest is where the label has least room, so that is where it gives.
  const shape = await page.evaluate(() => window.footerShape());
  expect(shape.counterClipped).toBe(true);
});

test("a narrow pane steps the type down and hides nothing", async ({ page }) => {
  await open(page);
  const wide = await page.evaluate(() => window.displays());
  const wideHeading = await page.evaluate(() => window.headingSize());

  await page.setViewportSize({ width: 400, height: 520 });
  const narrow = await page.evaluate(() => window.displays());
  const narrowHeading = await page.evaluate(() => window.headingSize());

  expect(narrow).toHaveLength(wide.length);
  const lost = wide.filter((display, at) => display !== "none" && narrow[at] === "none");
  expect(lost).toEqual([]);
  expect(narrowHeading).toBeLessThan(wideHeading);
});

test("a pushed document redraws the body without moving the reader", async ({ page }) => {
  await open(page, "?line=19&to=21");
  await expect.poll(() => shown(page)).toEqual(["2"]);
  await expect.poll(() => marked(page)).toEqual([19, 21]);

  await page.evaluate(() => window.pushEdited());
  await expect(page.locator('[data-screen="2"]')).toContainText("An edited middle paragraph.");
  await expect.poll(() => shown(page)).toEqual(["2"]);
});

test("a push that ticks the last box moves the meter, not the screen", async ({ page }) => {
  await open(page, "?line=19");
  await page.keyboard.press("ArrowRight");
  await expect.poll(() => shown(page)).toEqual(["3"]);

  await page.evaluate(() => window.pushFinished());
  await expect(page.locator('[data-count="3"]')).toHaveText("1 of 1 met");
  await expect.poll(() => shown(page)).toEqual(["3"]);
  expect(await marked(page)).toEqual([]);
});

const bar = (page) => page.evaluate(() => window.bar());

test("the bar appears when an agent is working and offers the meter's phase", async ({ page }) => {
  await open(page);
  await expect.poll(() => shown(page)).toEqual(["overview"]);
  expect(await bar(page)).toBeNull();

  await page.evaluate(() => window.emitChrome({ activity: "working" }));
  await expect.poll(() => bar(page)).toMatchObject({
    says: "Agent is on phase 2 · The middle",
    dot: RUNNING,
    follow: false,
  });
  // The bar names where the agent is; it does not move the reader off the
  // screen they opened on.
  expect(await shown(page)).toEqual(["overview"]);

  await follow(page).check();
  await expect.poll(() => shown(page)).toEqual(["2"]);
  await expect.poll(() => bar(page)).toMatchObject({
    says: "Following the agent · The middle",
    follow: true,
    // Blue, because the reader operates it; green already means met.
    followFill: ACTION,
  });
});

test("navigating unticks Follow and ticking it hands the position back", async ({ page }) => {
  await open(page);
  await page.evaluate(() => window.emitChrome({ activity: "working" }));
  await follow(page).check();
  await expect.poll(() => shown(page)).toEqual(["2"]);

  // The controls are not gated by the tick; using one reports through it.
  await page.keyboard.press("ArrowRight");
  await expect.poll(() => shown(page)).toEqual(["3"]);
  await expect.poll(() => bar(page)).toMatchObject({
    says: "Agent is on phase 2 · The middle",
    follow: false,
  });

  await page.getByRole("button", { name: "Overview" }).click();
  await expect.poll(() => shown(page)).toEqual(["overview"]);
  await expect.poll(() => bar(page)).toMatchObject({ follow: false });

  await follow(page).check();
  await expect.poll(() => shown(page)).toEqual(["2"]);
  await expect.poll(() => bar(page)).toMatchObject({
    says: "Following the agent · The middle",
    follow: true,
  });
});

test("unticking Follow pins the reader in place and lets the meter go on alone", async ({
  page,
}) => {
  await open(page);
  await page.evaluate(() => window.emitChrome({ activity: "working" }));
  await follow(page).check();
  await expect.poll(() => shown(page)).toEqual(["2"]);

  await follow(page).uncheck();
  expect(await shown(page)).toEqual(["2"]);
  await expect.poll(() => bar(page)).toMatchObject({
    says: "Agent is on phase 2 · The middle",
    follow: false,
  });

  await page.evaluate(() => window.pushSecondMet());
  await expect.poll(() => bar(page)).toMatchObject({ says: "Agent is on phase 3 · The end" });
  expect(await shown(page)).toEqual(["2"]);
});

// A screen counts the sections between the phases, so it is not a phase number.
// Every earlier fixture opened straight into phase 1, where the two agree.
test("the bar names the meter's phase on a deck with sections before it", async ({ page }) => {
  await open(page, "?leading=true");
  await page.evaluate(() => window.emitChrome({ activity: "working" }));
  await expect.poll(() => bar(page)).toMatchObject({
    says: "Agent is on phase 1 · Write the greeting",
  });

  // Read from a slide of the reader's own, so the copy names the meter's phase
  // rather than the one they happen to be on.
  await page.keyboard.press("ArrowRight");
  await expect.poll(() => shown(page)).toEqual(["Decisions"]);
  await page.evaluate(() => window.pushLeading("second"));
  await expect.poll(() => bar(page)).toMatchObject({
    says: "Agent is on phase 2 · Turn it into a script",
  });
  expect(await page.evaluate(() => window.errors)).toEqual([]);
});

// The last phase is where a lookup running off the end of the phases stops
// naming anything and the pane quietly stops moving.
test("the meter reaching the last phase names it and takes the reader there", async ({ page }) => {
  await open(page, "?leading=true");
  await page.evaluate(() => window.emitChrome({ activity: "working" }));
  await follow(page).check();
  await expect.poll(() => shown(page)).toEqual(["1"]);

  await page.evaluate(() => window.pushLeading("last"));
  await expect.poll(() => bar(page)).toMatchObject({
    says: "Following the agent · Count to three",
    follow: true,
  });
  await expect.poll(() => shown(page)).toEqual(["3"]);
  expect(await page.evaluate(() => window.counter())).toBe("3 / 3");

  // And the footer is not a second opinion about which slide is up: navigating
  // past the last phase takes the counter and the underline with it.
  await page.keyboard.press("ArrowRight");
  await expect.poll(() => shown(page)).toEqual(["Blockers"]);
  expect(await page.evaluate(() => window.counter())).toBe("Blockers");
  expect((await page.evaluate(() => window.pips())).filter((pip) => pip.viewing)).toEqual([
    { label: "Blockers", viewing: true },
  ]);
  expect(await page.evaluate(() => window.errors)).toEqual([]);
});

// Three sections ahead of the phases, as P001 has, is where the old lookup ran
// past the end of the array and took the whole pane down with it.
test("a deck with three sections before its phases still draws", async ({ page }) => {
  await open(page, "?deep=true");
  await page.evaluate(() => window.emitChrome({ activity: "working" }));
  await expect.poll(() => bar(page)).toMatchObject({
    says: "Agent is on phase 1 · Write the greeting",
  });
  expect(await page.evaluate(() => window.errors)).toEqual([]);
});

// Nothing but sections is a deck with no meter, so the bar has nothing to say.
test("a deck of sections alone offers no bar", async ({ page }) => {
  await open(page, "?sections=true");
  await page.evaluate(() => window.emitChrome({ activity: "working" }));
  await expect.poll(() => shown(page)).toEqual(["overview"]);
  expect(await bar(page)).toBeNull();
  expect(await page.evaluate(() => window.errors)).toEqual([]);
});

test("a push that meets the last phase turns the bar green", async ({ page }) => {
  await open(page);
  await page.evaluate(() => window.emitChrome({ activity: "working" }));
  await follow(page).check();
  await expect.poll(() => shown(page)).toEqual(["2"]);

  await page.evaluate(() => window.pushFinished());
  // Nothing left to follow, so nothing is offered.
  await expect.poll(() => bar(page)).toMatchObject({
    says: "Every phase met",
    dot: "rgb(166, 218, 149)",
    follow: null,
  });
  await expect.poll(() => shown(page)).toEqual(["3"]);
});

test("a span still wins while an agent is working", async ({ page }) => {
  await open(page, "?line=41");
  await expect.poll(() => shown(page)).toEqual(["3"]);

  await page.evaluate(() => window.emitChrome({ activity: "working" }));
  await expect.poll(() => bar(page)).toMatchObject({ follow: false });
  expect(await shown(page)).toEqual(["3"]);
  expect(await marked(page)).toEqual([41]);

  const intervals = await page.evaluate(() => window.reports.at(-1).intervals);
  expect(intervals.some((interval) => interval.line <= 41 && interval.to >= 41)).toBe(true);

  await follow(page).check();
  await expect.poll(() => shown(page)).toEqual(["2"]);
});

test("phases sharing a number still render, each reporting its own", async ({ page }) => {
  await open(page);
  await page.evaluate(() => window.pushRenumbered());

  await expect.poll(() => page.evaluate(() => window.counters())).toEqual([
    "1 Groundwork 2/2",
    "2 The middle 1/2",
    "1 The end 0/1",
  ]);
  expect(await page.evaluate(() => window.errors)).toEqual([]);

  // Both slides say Phase 1 because the document does; the pane reports the
  // number it was given rather than inventing a position.
  await page.locator('.pip[aria-label="Phase 1"]').last().click();
  await expect.poll(() => page.evaluate(() => window.counter())).toBe("1 / 3");
  await expect.poll(() => page.evaluate(() => window.crumbs())).toContain("Phase 1 of 3");
});

test("arrow keys are left to the plain body when nothing opens a phase", async ({ page }) => {
  await open(page, "?plain=true");
  await page.keyboard.press("ArrowRight");
  await expect(page.locator("[data-screen]")).toHaveCount(0);
  expect(await page.evaluate(() => window.errors)).toEqual([]);
});

test("a finished plan still opens on the overview", async ({ page }) => {
  await open(page, "?done=true");

  await expect.poll(() => shown(page)).toEqual(["overview"]);
  await expect.poll(() => page.evaluate(() => window.counter())).toBe("Overview");
  await expect.poll(() => bar(page)).toMatchObject({ says: "Every phase met", follow: null });
  expect((await page.evaluate(() => window.pips()))[0]).toEqual({ label: "Overview", viewing: true });
});

test("the counter and the pip name the screen being viewed, not the meter", async ({ page }) => {
  await open(page, "?done=true");
  expect(await page.evaluate(() => window.counter())).toBe("Overview");

  for (let phase = 1; phase <= 6; phase++) {
    await page.keyboard.press("ArrowRight");
    await expect.poll(() => shown(page)).toEqual([String(phase)]);
    expect(await page.evaluate(() => window.counter())).toBe(`${phase} / 6`);
    expect(await page.evaluate(() => window.crumbs())).toContain(`Phase ${phase} of 6`);
    const viewing = (await page.evaluate(() => window.pips())).filter((pip) => pip.viewing);
    expect(viewing).toEqual([{ label: `Phase ${phase}`, viewing: true }]);
  }

  await page.locator('.pip[aria-label="Overview"]').click();
  await expect.poll(() => shown(page)).toEqual(["overview"]);
  expect(await page.evaluate(() => window.counter())).toBe("Overview");
});

test("the footer holds the pane's floor and spans its width", async ({ page }) => {
  await open(page, "?done=true");
  // Phase 6 is the shortest screen; the footer must not ride up under it.
  await page.locator('.pip[aria-label="Phase 6"]').click();
  await expect.poll(() => shown(page)).toEqual(["6"]);

  const short = await page.evaluate(() => window.footerGap());
  expect(short).toEqual({ gap: 0, left: 0, width: short.pane, pane: short.pane });

  // And on the longest screen, where the deck scrolls underneath it.
  await page.locator('.pip[aria-label="Overview"]').click();
  await expect.poll(() => shown(page)).toEqual(["overview"]);
  const long = await page.evaluate(() => window.footerGap());
  expect(long).toEqual({ gap: 0, left: 0, width: long.pane, pane: long.pane });
});

test("following moves the view when the meter moves, and a pin holds it", async ({ page }) => {
  await open(page);
  await page.evaluate(() => window.emitChrome({ activity: "working" }));
  await expect.poll(() => shown(page)).toEqual(["overview"]);

  // Phase 2's last box gets ticked, so the meter moves on to phase 3.
  await page.evaluate(() => window.pushFinished());
  await expect.poll(() => shown(page)).toEqual(["3"]);
  expect(await page.evaluate(() => window.counter())).toBe("3 / 3");
});

test("sections on both sides of the phases are slides of their own", async ({ page }) => {
  await open(page, "?around=true");

  // Four slides: a leading section, two phases, a trailing one.
  expect(await page.evaluate(() => window.pipKinds())).toEqual([
    { label: "Overview", outline: true },
    { label: "Decisions", outline: true },
    { label: "Phase 1", outline: false },
    { label: "Phase 2", outline: false },
    { label: "Blockers", outline: true },
  ]);

  // The trailing section is not swallowed into the last phase's body.
  await page.locator('.pip[aria-label="Phase 2"]').click();
  await expect(page.locator('[data-screen="2"]')).not.toContainText("of the blockers");
  await page.locator('.pip[aria-label="Blockers"]').click();
  await expect(page.locator('[data-screen="Blockers"]')).toContainText("of the blockers");
});

test("the counter reads a phase in phases and a section by name", async ({ page }) => {
  await open(page, "?around=true");
  expect(await page.evaluate(() => window.counter())).toBe("Overview");

  await page.locator('.pip[aria-label="Decisions"]').click();
  expect(await page.evaluate(() => window.counter())).toBe("Decisions");

  // Slide three of five, but phase one of two.
  await page.locator('.pip[aria-label="Phase 1"]').click();
  expect(await page.evaluate(() => window.counter())).toBe("1 / 2");
  expect(await page.evaluate(() => window.crumbs())).toContain("Phase 1 of 2");

  await page.locator('.pip[aria-label="Blockers"]').click();
  expect(await page.evaluate(() => window.counter())).toBe("Blockers");
});

test("a section screen carries no meter", async ({ page }) => {
  await open(page, "?around=true");
  await page.locator('.pip[aria-label="Decisions"]').click();
  await expect(page.locator('[data-screen="Decisions"] .criteria')).toHaveCount(0);
  await expect(page.locator('[data-screen="Decisions"] .crumb')).toHaveCount(0);
  await page.locator('.pip[aria-label="Phase 1"]').click();
  await expect(page.locator('[data-screen="1"] .criteria')).toHaveCount(1);
});

test("the document view keeps the strip, which follows the scroll", async ({ page }) => {
  await open(page, "?around=true");
  await page.getByRole("button", { name: "Document", exact: true }).click();
  await expect(page.locator(".footer")).toBeVisible();
  await expect(page.locator('button[aria-label="Next screen"]')).toHaveCount(0);

  // Clicking a pip scrolls to that section, and the strip reports where we are.
  await page.locator('.pip[aria-label="Blockers"]').click();
  await expect.poll(() => page.evaluate(() => window.counter())).toBe("Blockers");
  await expect
    .poll(() => page.evaluate(() => window.pips().filter((pip) => pip.viewing)))
    .toEqual([{ label: "Blockers", viewing: true }]);

  await page.evaluate(() => window.scrollTo_(0));
  await expect.poll(() => page.evaluate(() => window.counter())).toBe("Overview");
});
