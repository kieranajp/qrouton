import { createViewportController } from "../src/lib/panes/viewport.js";

const root = document.querySelector("#root");
const content = document.querySelector("#content");
const surface = document.querySelector("#surface");
const target = document.querySelector("#target");
const before = document.querySelector("#before");
const params = new URLSearchParams(location.search);

if (params.get("hidden") === "true") surface.style.display = "none";

window.reports = [];
window.reveals = 0;
const reveal = target.scrollIntoView.bind(target);
target.scrollIntoView = (options) => {
  window.reveals++;
  reveal(options);
};
window.controller = createViewportController({
  root,
  content,
  blocks: [...content.querySelectorAll("[data-line]")],
  target,
  span: { line: 4, to: 4 },
  selected: params.get("selected") !== "false",
  report: (value) => window.reports.push(value),
});
window.measure = () => window.controller.schedule();
window.setSelected = (value) => window.controller.setSelected(value);
window.activate = () => {
  surface.style.display = "block";
  window.controller.setSelected(true);
};
window.moveTargetWithReflow = () => {
  before.style.height = `${before.offsetHeight + 180}px`;
  root.style.height = `${root.offsetHeight - 10}px`;
  document.fonts.dispatchEvent(new Event("loadingdone"));
};
window.scrollToTargetEdges = () => {
  root.scrollTop = target.offsetTop + 20;
};
window.scrollToNested = () => {
  root.scrollTop = document.querySelector("#nested").offsetTop + 10;
};
window.scrollToGap = () => {
  root.scrollTop = document.querySelector("#gap").offsetTop + 40;
};
window.resizeAfterScroll = () => {
  before.style.height = `${before.offsetHeight + 60}px`;
  root.style.width = `${root.offsetWidth - 20}px`;
};
