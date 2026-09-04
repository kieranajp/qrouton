import { renderDeck } from "../src/lib/panes/slides.js";

const parse = (html) => new DOMParser().parseFromString(html, "text/html");

window.deck = (markdown) => {
  const rendered = renderDeck(markdown);
  const parsed = parse(rendered.html);
  const container = parsed.querySelector("div.marpit");
  return {
    html: rendered.html,
    css: rendered.css,
    comments: rendered.comments,
    children: [...(container?.children ?? [])].map((element) => ({
      tag: element.tagName.toLowerCase(),
      classes: [...element.classList],
    })),
    media: [...parsed.querySelectorAll("img, video")].map((element) => ({
      tag: element.tagName.toLowerCase(),
      src: element.getAttribute("src"),
    })),
  };
};
