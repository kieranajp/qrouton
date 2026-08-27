import { activateMatch, clearMatches, markMatches } from "../src/lib/find.js";

const root = document.querySelector("#root");
const content = document.querySelector("#content");
let matches = [];
let current = -1;

window.searchDocument = (query) => {
  matches = markMatches(content, query);
  current = activateMatch(matches, 0);
  return matches.length;
};
window.moveMatch = (by) => {
  current = activateMatch(matches, current + by);
  return current;
};
window.findState = () => ({
  current,
  scrollTop: root.scrollTop,
  marks: [...content.querySelectorAll("mark")].map((mark) => ({
    text: mark.textContent,
    current: mark.classList.contains("current"),
  })),
});
window.clearDocumentSearch = () => clearMatches(content);
