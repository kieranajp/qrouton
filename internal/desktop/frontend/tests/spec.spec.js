import { expect, test } from "@playwright/test";

const open = async (page, query = "") => {
  await page.goto("/tests/spec.html" + query);
  await page.waitForSelector("[data-screen], .markdown", { state: "attached" });
};

const shown = (page) => page.evaluate(() => window.shown());

test("the overview counts the answered questions and lists the decisions", async ({ page }) => {
  await open(page);
  await expect.poll(() => shown(page)).toEqual(["overview"]);
  await expect(page.locator("[data-tally]")).toContainText("1 of 3 answered");
  await expect(page.locator(".rows .row")).toHaveText([/Decisions\s*Keep `?Retry`? as a wrapper\s*Cancellation is cooperative/]);
});

test("the overview lists the open questions before the decisions", async ({ page }) => {
  await open(page);
  const labels = page.locator('[data-screen="overview"] .caps[data-list]');
  await expect(labels).toHaveText(["Open questions", "Decisions"]);
  const asks = page.locator("[data-open-list] .row");
  await expect(asks).toHaveCount(3);
  await expect(asks.nth(0)).toContainText("Q1");
  await expect(asks.nth(0)).toContainText("Answered");
  await expect(asks.nth(1)).toContainText("Q2");
  await expect(asks.nth(1).locator(".ask-state")).toHaveText("Open");
  const order = await page.evaluate(() => {
    const list = document.querySelector("[data-open-list]");
    const rows = document.querySelector(".rows");
    return Boolean(list.compareDocumentPosition(rows) & Node.DOCUMENT_POSITION_FOLLOWING);
  });
  expect(order).toBe(true);
});

test("clicking an open question on the overview lands on its card", async ({ page }) => {
  await open(page);
  await page.locator('[data-ask="Q3"]').click();
  await expect.poll(() => shown(page)).toEqual(["questions"]);
  await expect.poll(() => page.evaluate(() => window.focusedCard())).toBe("Q3");
  await expect(page.locator('[data-question="Q3"]')).toBeInViewport();
});

test("answering a question updates its row on the overview", async ({ page }) => {
  await open(page);
  await page.getByRole("button", { name: "Next open question" }).click();
  await page.locator('[data-question="Q2"]').getByRole("button", { name: /The context's error/ }).click();
  await expect(page.locator('[data-question="Q2"] .state')).toHaveText("Saved");
  await page.keyboard.press("ArrowLeft");
  await expect.poll(() => shown(page)).toEqual(["overview"]);
  await expect(page.locator('[data-ask="Q2"] .ask-state')).toHaveText("Answered B");
  await expect(page.locator('[data-ask="Q2"]')).toHaveClass(/settled/);
  await expect(page.locator("[data-tally]")).toContainText("2 of 3 answered");
});

test("the questions pip holds every open question as a card", async ({ page }) => {
  await open(page);
  await page.getByRole("button", { name: "Next open question" }).click();
  await expect.poll(() => shown(page)).toEqual(["questions"]);

  const cards = page.locator('[data-screen="questions"] .card');
  await expect(cards).toHaveCount(3);
  await expect(cards.nth(0).locator(".option")).toHaveCount(3);
  await expect(cards.nth(0).locator(".option.chosen")).toContainText("Overall");
  await expect(cards.nth(0).locator(".badge")).toHaveText("Recommended");
  await expect(cards.nth(0).locator(".reason")).toContainText("caps the caller's wait");
  await expect.poll(() => page.evaluate(() => window.focusedCard())).toBe("Q2");
});

test("bold decisions share one pip, after the questions", async ({ page }) => {
  await open(page);
  expect(await page.evaluate(() => window.pips())).toEqual([
    "Overview",
    "Open questions",
    "Decisions",
    "End state",
    "Risks",
  ]);
});

test("arrow keys move between pips, and up and down between cards", async ({ page }) => {
  await open(page);
  await page.keyboard.press("ArrowRight");
  await expect.poll(() => shown(page)).toEqual(["questions"]);
  await expect.poll(() => page.evaluate(() => window.focusedCard())).toBe("Q1");

  await page.keyboard.press("ArrowDown");
  await expect.poll(() => page.evaluate(() => window.focusedCard())).toBe("Q2");
  await page.keyboard.press("j");
  await expect.poll(() => page.evaluate(() => window.focusedCard())).toBe("Q3");
  await page.keyboard.press("k");
  await expect.poll(() => page.evaluate(() => window.focusedCard())).toBe("Q2");

  await page.keyboard.press("ArrowRight");
  await expect.poll(() => shown(page)).toEqual(["Decisions"]);
});

test("a spec with nothing open says so and has no questions pip", async ({ page }) => {
  await open(page, "?zero=1");
  await expect(page.locator("[data-tally]")).toHaveText("No open questions");
  expect(await page.evaluate(() => window.pips())).toEqual([
    "Overview",
    "Decisions",
  ]);
  await expect(page.locator('[data-screen="overview"]')).not.toContainText("None.");
  await expect(page.locator("[data-open-list]")).toHaveCount(0);
});

test("topic groups page the decisions by heading", async ({ page }) => {
  await open(page, "?clef=1");
  expect(await page.evaluate(() => window.pips())).toEqual([
    "Overview",
    "Open questions",
    "Product",
    "The Clef call",
    "Storage",
  ]);
  await expect(page.locator('[data-screen="overview"] .caps').first()).toHaveText("Spec · 7 decisions");
  await expect(page.locator(".rows .row")).toHaveText([
    /Product · 3 decisions\s*Grams go entirely\s*The bars go\s*A failed estimate shows no dots/,
    /The Clef call · 2 decisions/,
    /Storage · 2 decisions/,
  ]);

  await page.locator(".rows .row").nth(1).click();
  await expect.poll(() => shown(page)).toEqual(["The Clef call"]);
  const group = page.locator('[data-screen="The Clef call"]');
  await expect(group).toContainText("nutrition-intelligence owns the call.");
  await expect(group).toContainText("lsx makes one call per entry.");
  await expect(group.locator(".markdown strong")).toHaveCount(2);
});

test("a Decisions section of plain bullets is one page that shows them", async ({ page }) => {
  await open(page, "?bullets=1");
  expect(await page.evaluate(() => window.pips())).toEqual(["Overview", "Decisions"]);
  await page.locator(".rows .row").click();
  await expect.poll(() => shown(page)).toEqual(["Decisions"]);
  const decisions = page.locator('[data-screen="Decisions"]');
  await expect(decisions).toContainText("Retries stop after three attempts");
  await expect(decisions).toContainText("Backoff is capped at ten seconds");
});

test("a spec in no known shape renders as plain markdown", async ({ page }) => {
  await open(page, "?freeform=1");
  await expect(page.locator(".pip")).toHaveCount(0);
  await expect(page.locator(".markdown").first()).toContainText("Do the thing the old way.");
});

test("the document toggle shows the whole text", async ({ page }) => {
  await open(page);
  await page.getByRole("button", { name: "Document" }).click();
  await expect(page.locator(".reading")).toContainText("Per attempt or overall?");
  await expect(page.locator(".reading")).toContainText("Answer: B");
});

test("cards fit the pane without scrolling sideways", async ({ page }) => {
  await page.setViewportSize({ width: 380, height: 800 });
  await open(page);
  await page.keyboard.press("ArrowRight");
  const deck = page.locator(".deck");
  const overflow = await deck.evaluate((el) => el.scrollWidth - el.clientWidth);
  expect(overflow).toBeLessThanOrEqual(0);
});

const card = (page, id) => page.locator(`[data-question="${id}"]`);
const questions = async (page) => {
  await open(page);
  await page.keyboard.press("ArrowRight");
  await expect.poll(() => shown(page)).toEqual(["questions"]);
};

test("one click saves the picked option into the file", async ({ page }) => {
  await questions(page);
  await card(page, "Q2").getByRole("button", { name: /The context's error/ }).click();

  await expect.poll(() => page.evaluate(() => window.saves().length)).toBe(1);
  const [[id, sent, text]] = await page.evaluate(() => window.saves());
  expect(id).toBe("w1");
  expect(sent).toMatch(/^h/);
  const open = await page.evaluate(() => window.OPEN);
  expect(text).toBe(open.replace("Answer:\n\n### Q3", "Answer: B\n\n### Q3"));
  await expect(card(page, "Q2").locator(".state")).toHaveText("Saved");
  await expect(page.locator(".crumb .count")).toHaveText("2 of 3 answered");
});

test("a typed note saves when the field loses focus", async ({ page }) => {
  await questions(page);
  const field = card(page, "Q3").locator("textarea");
  await field.fill("Only when the caller asks");
  await expect(card(page, "Q3").locator(".state")).toHaveText("Not saved");
  expect(await page.evaluate(() => window.saves().length)).toBe(0);

  await page.getByRole("heading", { name: "Open questions" }).click();
  await expect.poll(() => page.evaluate(() => window.saves().length)).toBe(1);
  const [[, , text]] = await page.evaluate(() => window.saves());
  expect(text).toContain("- B. No\n\nAnswer: Only when the caller asks\n\n## Decisions");
});

test("a refused save reloads the spec and keeps the typed answer", async ({ page }) => {
  await questions(page);
  const moved = await page.evaluate(() => window.OPEN + "Appended by the agent.\n");
  await page.evaluate((text) => window.staleOnce(text), moved);

  const field = card(page, "Q2").locator("textarea");
  await field.fill("kept draft");
  await field.press("ControlOrMeta+Enter");

  await expect(card(page, "Q2").locator(".message")).toContainText("changed on disk");
  await expect(field).toHaveValue("kept draft");

  await field.press("ControlOrMeta+Enter");
  await expect.poll(() => page.evaluate(() => window.saves().length)).toBe(2);
  const text = await page.evaluate(() => window.saves()[1][2]);
  expect(text).toContain("Appended by the agent.");
  expect(text).toContain("Answer: kept draft");
  await expect(card(page, "Q2").locator(".state")).toHaveText("Saved");
});

test("a draft whose question is renamed under it is kept as detached", async ({ page }) => {
  await questions(page);
  await card(page, "Q3").locator("textarea").fill("my unsaved thought");
  const renamed = await page.evaluate(() =>
    window.OPEN.replace("### Q3 — Is zero attempts an error?", "### Q3 — Should zero attempts fail?"),
  );
  await page.evaluate((text) => window.pushContent(text), renamed);

  await expect(page.locator('[data-orphan="Q3"]')).toContainText("my unsaved thought");
  await expect(page.locator('[data-orphan="Q3"]')).toContainText("Is zero attempts an error?");
  await expect(card(page, "Q3").locator("h2")).toHaveText("Should zero attempts fail?");
  await expect(card(page, "Q3").locator("textarea")).toHaveValue("");
});

test("a letter key picks that option on the focused card", async ({ page }) => {
  await questions(page);
  await page.keyboard.press("j");
  await page.keyboard.press("b");
  await expect.poll(() => page.evaluate(() => window.saves().length)).toBe(1);
  expect(await page.evaluate(() => window.saves()[0][2])).toContain("callers can test for it)**\n\nAnswer: B\n");
});

const sendButton = (page) => page.locator("[data-send]");
const why = (page) => page.locator(".send .why");

test("send waits until something is answered and saved", async ({ page }) => {
  await open(page, "?zero=1");
  await expect(sendButton(page)).toHaveCount(0);

  await page.evaluate(() => window.pushContent(window.OPEN.replace("Answer: B", "Answer:")));
  await expect(sendButton(page)).toBeDisabled();
  await expect(why(page)).toHaveText("Answer at least one question first.");
  await expect(page.locator(".send-button")).toHaveAttribute("title", "Answer at least one question first.");

  await page.keyboard.press("ArrowRight");
  const field = card(page, "Q1").locator("textarea");
  await field.fill("an unsaved note");
  await expect(sendButton(page)).toBeDisabled();
  await expect(why(page)).toContainText("Save your typed answer first");

  await page.evaluate(() => window.holdSaves());
  await field.press("ControlOrMeta+Enter");
  await expect(why(page)).toHaveText("Wait for the save to finish.");
  await page.evaluate(() => window.releaseSaves());
  await expect(sendButton(page)).toBeEnabled();
});

test("send is held back while a save has failed", async ({ page }) => {
  await questions(page);
  await page.evaluate(() => window.staleOnce(window.OPEN + "\nMoved.\n"));
  const field = card(page, "Q2").locator("textarea");
  await field.fill("kept");
  await field.press("ControlOrMeta+Enter");
  await expect(why(page)).toHaveText("An answer did not save. Save it again first.");
  await expect(sendButton(page)).toBeDisabled();
});

test("one click on send makes one bridge call with the window id", async ({ page }) => {
  await questions(page);
  await expect(sendButton(page)).toBeEnabled();
  await sendButton(page).click();
  await expect(why(page)).toHaveText("Typed into the conversation.");
  expect(await page.evaluate(() => window.sends())).toEqual([["w1"]]);
});

test("answering the last open question makes send the primary action", async ({ page }) => {
  await questions(page);
  await expect(sendButton(page)).toHaveClass(/outline/);
  await card(page, "Q2").locator(".option").first().click();
  await card(page, "Q3").locator(".option").first().click();
  await expect(page.locator(".crumb .count")).toHaveText("3 of 3 answered");
  await expect(sendButton(page)).toHaveClass(/primary/);
});

test("clicking the chosen option again clears the letter and keeps the note as the answer", async ({ page }) => {
  await questions(page);
  const q1 = card(page, "Q1");
  const note = "Neither: use an absolute caller deadline instead.";
  await q1.locator("textarea").fill(note);
  await q1.getByRole("button", { name: /Overall/ }).click();
  await expect(q1.getByRole("button", { name: /Overall/ })).toHaveAttribute("aria-pressed", "false");

  await expect.poll(() => page.evaluate(() => window.saves().length)).toBeGreaterThan(0);
  const last = () => page.evaluate(() => window.saves().at(-1)[2]);
  await expect.poll(last).toContain(`Answer: ${note}\n`);
  expect(await last()).not.toMatch(/Answer: B/);
  await expect(q1.locator(".state")).toHaveText("Saved");
});

test("a letter key on the chosen option clears it, and a note beside a letter still saves", async ({ page }) => {
  await questions(page);
  const q1 = card(page, "Q1");
  const overall = q1.getByRole("button", { name: /Overall/ });
  await expect(overall).toHaveAttribute("aria-pressed", "true");

  await page.keyboard.press("b");
  await expect(overall).toHaveAttribute("aria-pressed", "false");
  await expect.poll(() => page.evaluate(() => window.saves().at(-1)[2])).toContain("it caps the caller's wait)**\n- C. Both\n\nAnswer:\n");

  await page.keyboard.press("b");
  await expect(overall).toHaveAttribute("aria-pressed", "true");
  await q1.locator("textarea").fill("Because of the SLA");
  await page.getByRole("heading", { name: "Open questions" }).click();
  await expect.poll(() => page.evaluate(() => window.saves().at(-1)[2])).toContain("Answer: B\nBecause of the SLA\n");
});

test("a note that opens with a letter is saved as free text and no option is highlighted after a reload", async ({ page }) => {
  await questions(page);
  const q1 = card(page, "Q1");
  const note = "B, but only if X";
  await q1.getByRole("button", { name: /Overall/ }).click();
  await expect(q1.getByRole("button", { name: /Overall/ })).toHaveAttribute("aria-pressed", "false");
  await q1.locator("textarea").fill(note);
  await page.getByRole("heading", { name: "Open questions" }).click();

  const last = () => page.evaluate(() => window.saves().at(-1)[2]);
  await expect.poll(last).toContain(`\nAnswer: ${note}\n`);
  expect(await last()).not.toMatch(/^Answer: [A-Z]$/m);

  await page.evaluate((text) => window.pushContent(text), await last());
  await expect(q1.locator("[aria-pressed=true]")).toHaveCount(0);
  await expect(q1.locator("textarea")).toHaveValue(note);
});

test("a picked option and a note save as the letter on its own line with the note below", async ({ page }) => {
  await questions(page);
  const q3 = card(page, "Q3");
  await q3.getByRole("button", { name: /No/ }).click();
  await q3.locator("textarea").fill("only when the caller asks");
  await page.getByRole("heading", { name: "Open questions" }).click();

  await expect.poll(() => page.evaluate(() => window.saves().at(-1)[2])).toContain(
    "- B. No\n\nAnswer: B\nonly when the caller asks\n\n## Decisions",
  );
});

test("a d2 fence draws in a question's context, a decision page and another section", async ({ page }) => {
  await open(page, "?d2=1");
  const fence = (screen) => page.locator(`[data-screen="${screen}"] pre.diagram`);

  await page.getByRole("button", { name: "Next open question" }).click();
  await expect(fence("questions")).toHaveCount(1);
  await expect(page.locator('[data-screen="questions"] pre:not(.diagram) code')).toHaveCount(0);

  await page.getByRole("button", { name: "Next screen" }).click();
  await expect.poll(() => shown(page)).toEqual(["Decisions"]);
  await expect(fence("Decisions")).toHaveCount(1);

  await page.getByRole("button", { name: "Next screen" }).click();
  await page.getByRole("button", { name: "Next screen" }).click();
  await expect.poll(() => shown(page)).toEqual(["Risks"]);
  await expect(fence("Risks")).toHaveCount(1);
});

test("a saved answer leaves the question's diagram drawn", async ({ page }) => {
  await open(page, "?d2=1");
  await page.getByRole("button", { name: "Next open question" }).click();
  await page.locator('[data-question="Q2"] .option').first().click();
  await expect.poll(() => page.evaluate(() => window.saves().length)).toBe(1);
  await expect(page.locator('[data-question="Q1"] pre.diagram')).toHaveCount(1);
});
