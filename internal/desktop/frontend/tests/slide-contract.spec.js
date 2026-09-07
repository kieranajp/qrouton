import { expect, test } from "@playwright/test";

const deck = `---
marp: true
theme: no-such-theme
---

<!-- _class: title -->

# Opening

<!-- the note under the opening -->

---

## Second

<video src="./demo.mp4"></video>
<img src="./shot.png">

![](./shot.png)

---

## Third
`;

const render = async (page, markdown) => {
  await page.goto("/tests/slide-contract.html");
  return page.evaluate((source) => window.deck(source), markdown);
};

test("every slide is a section directly under the container", async ({ page }) => {
  const rendered = await render(page, deck);

  expect(rendered.children.map((child) => child.tag)).toEqual(["section", "section", "section"]);
  expect(rendered.html).not.toContain("<svg");
  expect(rendered.html).not.toContain("foreignObject");
});

test("a directive comment classes its slide and a plain one is a note", async ({ page }) => {
  const rendered = await render(page, deck);

  expect(rendered.children[0].classes).toContain("title");
  expect(rendered.comments).toEqual([["the note under the opening"], [], []]);
});

test("the theme is rewritten into the container's scope", async ({ page }) => {
  const rendered = await render(page, deck);

  expect(rendered.css).not.toContain(":root");
  expect(rendered.css).toContain("div.marpit > :where(section)");
  expect(rendered.css).toContain("var(--surface-app)");
});

test("a relative media path survives the allowlist untouched", async ({ page }) => {
  const rendered = await render(page, deck);

  expect(rendered.media).toEqual([
    { tag: "video", src: "./demo.mp4" },
    { tag: "img", src: "./shot.png" },
    { tag: "img", src: "./shot.png" },
  ]);
});

test("a deck naming an unknown theme still renders against ours", async ({ page }) => {
  const rendered = await render(page, deck);

  expect(rendered.children).toHaveLength(3);
  expect(rendered.css).toContain("var(--surface-app)");
});

test("inline style and event handlers do not survive the allowlist", async ({ page }) => {
  const rendered = await render(page, `<div class="cols" style="flex: 2" onclick="boom()">x</div>\n`);

  expect(rendered.html).toContain('<div class="cols">');
  expect(rendered.html).not.toContain("onclick");
  expect(rendered.html).not.toContain("flex: 2");
});
