import { renderDeck } from "../src/lib/panes/slides.js";

const DECK = `# Content

Body copy on the default layout.

---

<!-- _class: title -->

# Title

---

<!-- _class: statement -->

# Statement

---

<!-- _class: alt -->

# Alt

---

<div class="cols wide-left"><div>left</div><div>right</div></div>

<figure class="shot"><img src="/nothing.png"><figcaption>caption</figcaption></figure>

<div class="cards"><div>plain</div><div class="accent"><h3>accent</h3><p><strong>bold</strong> and <code>code</code></p></div></div>

<div class="callout">plain</div>
<div class="callout note">note</div>
<div class="callout good">good</div>
<div class="callout warn">warn</div>

<p class="note">A muted aside.</p>
`;

const rendered = renderDeck(DECK);
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
    border: computed.borderTopColor,
    edge: computed.borderLeftColor,
    family: computed.fontFamily,
    size: Number.parseFloat(computed.fontSize),
  };
};

window.slideBoxes = (selector) =>
  [...document.querySelectorAll(selector)].map((element) => {
    const rect = element.getBoundingClientRect();
    return { width: Math.round(rect.width), left: Math.round(rect.left) };
  });
