import "../src/tokens/typography.css";
import "../src/tokens/spacing.css";
import "../src/tokens/effects.css";
import { mount } from "svelte";
import SlidesFixture from "./SlidesFixture.svelte";
import { emitWailsEvent } from "./wails-runtime.js";

export const DECK = [
  "---", // 1
  "marp: true", // 2
  "---", // 3
  "", // 4
  "<!-- _class: title -->", // 5
  "", // 6
  "# The fixture deck", // 7
  "", // 8
  "<!-- The note under the opener, which the reader can see. -->", // 9
  "", // 10
  "---", // 11
  "", // 12
  "## Second", // 13
  "", // 14
  "Body copy on the second slide.", // 15
  "", // 16
  "---", // 17
  "", // 18
  "## Third", // 19
  "", // 20
  "A slide with a very long note beneath it.", // 21
  "", // 22
  `<!-- ${"A long note that runs on and on. ".repeat(40)} -->`, // 23
  "", // 24
  "---", // 25
  "", // 26
  "## Fourth", // 27
  "", // 28
  "---", // 29
  "", // 30
  "## Fifth", // 31
  "", // 32
  "---", // 33
  "", // 34
  "## Sixth", // 35
  "", // 36
  "---", // 37
  "", // 38
  "## Seventh", // 39
  "", // 40
  "Body copy on the seventh slide.", // 41
  "", // 42
].join("\n");

const params = new URLSearchParams(location.search);
const number = (name) => Number(params.get(name) ?? 0);

const document_ = (text) => ({
  text,
  format: "markdown",
  source: "thoughts/shared/decks/fixture.md",
  path: "/sessions/fixture/thoughts/shared/decks/fixture.md",
  kind: "NOTE",
  deck: true,
  line: number("line"),
  to: number("to"),
  viewportEpoch: 1,
});

window.reports = [];
window.pushDeck = (text) => emitWailsEvent("window:content:w1", document_(text));
window.wailsCall = async (name, id, payload) => {
  if (name.endsWith(".Content")) return document_(DECK);
  if (name.endsWith(".ReportViewport")) window.reports.push(payload);
  return undefined;
};

const box = (element) => {
  const rect = element.getBoundingClientRect();
  return { width: rect.width, height: rect.height, top: rect.top };
};

window.cards = () =>
  [...document.querySelectorAll(".card")].map((card) => ({
    line: Number(card.dataset.line),
    lineEnd: Number(card.dataset.lineEnd),
    frame: box(card.querySelector(".frame")),
    notes: card.querySelector(".notes")?.getBoundingClientRect().height ?? 0,
  }));

window.counter = () => document.querySelector(".counter").textContent.trim();

window.narrow = () => {
  document.querySelector("#fixture").style.width = "480px";
};

window.scroller = () => document.querySelector(".body");

mount(SlidesFixture, { target: document.querySelector("#fixture") });
