import { renderDeck } from "../src/lib/panes/slides.js";

const deck = `# Slide\n`;

const rendered = renderDeck(deck);
const sheet = document.createElement("style");
sheet.textContent = rendered.css;
document.head.append(sheet);
document.querySelector("#deck-root").innerHTML = rendered.html;

window.slideStyle = (selector) => {
  const element = document.querySelector(selector);
  if (!element) return null;
  const computed = getComputedStyle(element);
  return {
    background: computed.backgroundColor,
    color: computed.color,
    borderColor: computed.borderTopColor,
  };
};
