import { expect, test } from "@playwright/test";

const open = async (page, query = "") => {
  await page.goto(`/tests/audit.html${query}`);
  await page.waitForSelector("[data-overview], .markdown", { state: "attached" });
};

const card = (page, id) => page.locator(`[data-finding="${id}"]`);
const sendButton = (page) => page.locator("[data-send]");
const why = (page) => page.locator(".send .why");

test("the overview tallies findings by tell, verdict, impact and triage", async ({ page }) => {
  await open(page);
  await expect(page.locator("[data-tally]")).toHaveText("1 of 4 triaged");
  await expect(page.locator('[data-group="By tell"] li')).toHaveText([
    /Duplicated behaviour or facts\s*1/,
    /Dead or needlessly defensive code\s*2/,
    /Homegrown library or platform behaviour\s*0/,
    /Tests that cannot fail or pass for the wrong reason\s*1/,
  ]);
  await expect(page.locator('[data-group="By verdict"] li')).toHaveText([
    /Step 1 · No tie\s*1/,
    /Step 3 · The guarded case cannot occur\s*2/,
    /Step 7 · The test cannot fail\s*1/,
  ]);
  await expect(page.locator('[data-group="By impact"] li')).toHaveText([/high\s*1/, /medium\s*1/, /low\s*1/, /unrated\s*1/]);
  await expect(page.locator('[data-group="By triage"] li')).toHaveText([/Accepted\s*0/, /Rejected\s*1/, /Deferred\s*0/, /Note only\s*0/, /Untriaged\s*3/]);
  await expect(page.locator("[data-tells-run]")).toHaveCount(0);
});

test("cards run from the highest impact to the lowest, confidence breaking ties, and show their fields", async ({ page }) => {
  await open(page);
  await expect(page.locator(".card")).toHaveCount(4);
  expect(await page.locator(".card").evaluateAll((cards) => cards.map((el) => el.dataset.finding))).toEqual(["F1", "F4", "F2", "F3"]);
  await expect(card(page, "F1").locator(".impact")).toHaveText("high impact");
  await expect(card(page, "F3").locator(".impact")).toHaveText("unrated impact");
  await expect(card(page, "F1").locator(".locations code")).toHaveText(["src/api/http.ts:41", "src/sync/queue.ts:88"]);
  await expect(card(page, "F1")).toContainText("with a timer that rejects.");
  await expect(card(page, "F1").locator('[data-choice="reject"]')).toHaveAttribute("aria-pressed", "true");
  await expect(card(page, "F1").locator("textarea")).toHaveValue("the copy in sync/ is kept for the offline build");
});

test("a triage choice saves into the file through SaveAnswers", async ({ page }) => {
  await open(page);
  await card(page, "F2").locator('[data-choice="accept"]').click();
  await expect.poll(() => page.evaluate(() => window.saves().length)).toBe(1);
  const [[id, hash, text]] = await page.evaluate(() => window.saves());
  expect(id).toBe("w1");
  expect(hash).toMatch(/^h/);
  const audit = await page.evaluate(() => window.AUDIT);
  expect(text).toBe(audit.replace("Source: llm\n\nTriage:\n\n### F3", "Source: llm\n\nTriage: accept\n\n### F3"));
  await expect(card(page, "F2").locator(".state")).toHaveText("Saved");
  await expect(page.locator("[data-tally]")).toHaveText("2 of 4 triaged");
});

test("a typed note saves when the field loses focus", async ({ page }) => {
  await open(page);
  const field = card(page, "F3").locator("textarea");
  await field.fill("maybe after the release");
  await expect(card(page, "F3").locator(".state")).toHaveText("Not saved");
  expect(await page.evaluate(() => window.saves().length)).toBe(0);

  await page.locator("[data-tally]").click();
  await expect.poll(() => page.evaluate(() => window.saves().length)).toBe(1);
  const [[, , text]] = await page.evaluate(() => window.saves());
  expect(text).toContain("Source: llm\n\nTriage: maybe after the release\n\n## Homegrown");
  await expect(card(page, "F3").locator(".state")).toHaveText("Saved");
});

test("a refused save reloads the audit and keeps the typed triage", async ({ page }) => {
  await open(page);
  const moved = await page.evaluate(() => window.AUDIT + "Appended by the agent.\n");
  await page.evaluate((text) => window.staleOnce(text), moved);

  const field = card(page, "F2").locator("textarea");
  await field.fill("kept draft");
  await field.press("ControlOrMeta+Enter");

  await expect(card(page, "F2").locator(".message")).toContainText("changed on disk");
  await expect(field).toHaveValue("kept draft");
  await expect(why(page)).toHaveText("A triage did not save. Save it again first.");
  await expect(sendButton(page)).toBeDisabled();

  await field.press("ControlOrMeta+Enter");
  await expect.poll(() => page.evaluate(() => window.saves().length)).toBe(2);
  const text = await page.evaluate(() => window.saves()[1][2]);
  expect(text).toContain("Appended by the agent.");
  expect(text).toContain("Triage: kept draft");
  await expect(card(page, "F2").locator(".state")).toHaveText("Saved");
});

test("a retitled finding keeps its unsaved triage", async ({ page }) => {
  await open(page);
  await card(page, "F3").locator("textarea").fill("my unsaved thought");
  const retitled = await page.evaluate(() =>
    window.AUDIT.replace("### F3 — A second clamp on the backoff", "### F3 — The backoff is clamped twice"),
  );
  await page.evaluate((text) => window.pushContent(text), retitled);
  await expect(card(page, "F3").locator("h2")).toHaveText("The backoff is clamped twice");
  await expect(card(page, "F3").locator("textarea")).toHaveValue("my unsaved thought");
  await expect(page.locator("[data-detached]")).toHaveCount(0);
});

test("send waits for every save, then makes one bridge call", async ({ page }) => {
  await open(page);
  await expect(sendButton(page)).toHaveText("Type 'plan the accepted findings' into the conversation");
  await expect(sendButton(page)).toBeEnabled();

  const field = card(page, "F3").locator("textarea");
  await field.fill("an unsaved note");
  await expect(sendButton(page)).toBeDisabled();
  await expect(why(page)).toContainText("Save your typed note first");

  await page.evaluate(() => window.holdSaves());
  await field.press("ControlOrMeta+Enter");
  await expect(why(page)).toHaveText("Wait for the save to finish.");
  await page.evaluate(() => window.releaseSaves());
  await expect(sendButton(page)).toBeEnabled();

  await sendButton(page).click();
  await expect(why(page)).toHaveText("Typed into the conversation.");
  expect(await page.evaluate(() => window.sends())).toEqual([["w1"]]);
});

test("send waits until something is triaged", async ({ page }) => {
  await open(page);
  await page.evaluate(() => window.pushContent(window.AUDIT.replace("Triage: reject\nthe copy in sync/ is kept for the offline build", "Triage:")));
  await expect(page.locator("[data-tally]")).toHaveText("0 of 4 triaged");
  await expect(sendButton(page)).toBeDisabled();
  await expect(why(page)).toHaveText("Triage at least one finding first.");
});

test("the document toggle shows the whole text", async ({ page }) => {
  await open(page);
  await page.getByRole("button", { name: "Document" }).click();
  await expect(page.locator(".reading")).toContainText("Two timeout helpers race a promise against a timer");
  await expect(page.locator(".reading")).toContainText("Triage: reject");
});

test("cards fit the pane without scrolling sideways", async ({ page }) => {
  await page.setViewportSize({ width: 380, height: 800 });
  await open(page);
  const overflow = await page.locator(".sheet").evaluate((el) => el.scrollWidth - el.clientWidth);
  expect(overflow).toBeLessThanOrEqual(0);
});

test("a subset audit tallies and cards only the tells it ran", async ({ page }) => {
  await open(page, "?subset");
  await expect(page.locator('[data-group="By tell"] li')).toHaveText([
    /Dead or needlessly defensive code\s*2/,
    /Tests that cannot fail or pass for the wrong reason\s*1/,
  ]);
  await expect(page.locator("[data-tells-run]")).toHaveText(
    "Ran 2 of 4 tells: Dead or needlessly defensive code, Tests that cannot fail or pass for the wrong reason",
  );
  await expect(card(page, "F1")).toHaveCount(0);
  await expect(page.locator('[data-section="Duplicated behaviour or facts"]')).toContainText("Two timeout helpers race a promise against a timer");
});

test("an audit from before the tells key shows its three tells and names them", async ({ page }) => {
  await open(page, "?legacy");
  await expect(page.locator('[data-group="By tell"] li')).toHaveText([
    /Duplicated behaviour or facts\s*1/,
    /Dead or needlessly defensive code\s*2/,
    /Homegrown library or platform behaviour\s*0/,
  ]);
  await expect(page.locator("[data-tells-run]")).toHaveText(
    "Ran 3 of 4 tells: Duplicated behaviour or facts, Dead or needlessly defensive code, Homegrown library or platform behaviour",
  );
});
