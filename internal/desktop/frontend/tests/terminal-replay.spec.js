import { expect, test } from "@playwright/test";

const queries = "\x1b[c\x1b[0c\x1b[>c\x1b[>0c\x1b[5n\x1b[6n\x1b[?6n\x1b]11;?\x07\x1b]11;?\x1b\\";
const setup = async (page) => {
  await page.goto("/tests/terminal-rendering.html");
  await page.waitForFunction(() => window.fixture);
  await page.evaluate(() => fixture.create());
};
const completed = (page, count = 1) => expect.poll(() => page.evaluate(() => fixture.terminals[0].completed)).toBe(count);

test("covered queries reply live and stay silent across ordered replays", async ({ page }) => {
  await setup(page);
  await page.evaluate((queries) => {
    fixture.send(0, "before" + queries, false, false);
    fixture.send(0, queries, true);
    fixture.send(0, "", true);
    fixture.send(0, "old", true);
    fixture.send(0, "\x1b[31mhistory" + queries, true);
    fixture.send(0, " live" + queries);
    fixture.done(0);
  }, queries);
  await completed(page);
  const result = await page.evaluate(() => ({ replies: fixture.terminals[0].replies, lines: fixture.lines(0),
    color: fixture.terminals[0].term.buffer.active.getLine(0).getCell(0).getFgColor() }));
  expect(result.replies).toHaveLength(18);
  expect(result.replies.filter((s) => s.endsWith("R"))).toEqual(["\x1b[1;7R", "\x1b[?1;7R", "\x1b[1;13R", "\x1b[?1;13R"]);
  expect(result.lines[0]).toBe("history live");
  expect(result.color).toBe(1);
});

test("keyboard, paste and another terminal work while replay parsing is held", async ({ page }) => {
  await setup(page);
  await page.evaluate((queries) => {
    fixture.create();
    fixture.hold(0);
    fixture.send(0, "\x1b]778;hold\x07" + queries, true);
    fixture.send(1, queries);
    fixture.done(1);
  }, queries);
  await page.waitForFunction(() => fixture.terminals[0].held && fixture.terminals[1].completed === 1);
  await page.evaluate(() => fixture.terminals[0].term.focus());
  await page.keyboard.type("hello");
  await page.evaluate(() => fixture.terminals[0].term.paste("pasted"));
  expect(await page.evaluate(() => fixture.terminals[0].replies.join(""))).toBe("hellopasted");
  expect(await page.evaluate(() => fixture.terminals[1].replies.length)).toBe(9);
  await page.evaluate(() => { fixture.terminals[0].release(); fixture.done(0); });
  await completed(page);
  expect(await page.evaluate(() => fixture.terminals[0].replies.join(""))).toBe("hellopasted");
});

test("replay preserves background assignments and delegates other query forms", async ({ page }) => {
  await setup(page);
  await page.evaluate(() => {
    fixture.send(0, "\x1b]11;#123456\x07\x1b]11;?\x07", true);
    fixture.send(0, "\x1b]11;?\x07");
    fixture.done(0);
  });
  await completed(page);
  expect(await page.evaluate(() => fixture.terminals[0].replies)).toEqual(["\x1b]11;rgb:1212/3434/5656\x1b\\"]);
  await page.evaluate(() => {
    fixture.send(0, "\x1b]11;?;?\x07", true);
    fixture.done(0);
  });
  await completed(page, 2);
  expect(await page.evaluate(() => fixture.terminals[0].replies)).toHaveLength(3);
});

test("disposal abandons queued output and cannot affect a fresh mount", async ({ page }) => {
  await setup(page);
  await page.evaluate(() => { fixture.hold(0); fixture.send(0, "\x1b]778;hold\x07", true); fixture.send(0, "queued"); });
  await page.waitForFunction(() => fixture.terminals[0].held);
  await page.evaluate(() => {
    const entry = fixture.terminals[0];
    entry.dispose();
    entry.lateWrites = 0;
    entry.term.write = () => { entry.lateWrites++; };
    entry.release();
    fixture.send(0, "late");
    fixture.create();
    fixture.send(1, "fresh\x1b[c");
    fixture.done(1);
  });
  await page.waitForFunction(() => fixture.terminals[1].completed === 1);
  expect(await page.evaluate(() => fixture.terminals[0].lateWrites)).toBe(0);
  expect(await page.evaluate(() => fixture.lines(1)[0])).toBe("fresh");
  expect(await page.evaluate(() => fixture.terminals[1].replies)).toEqual(["\x1b[?1;2c"]);
});
