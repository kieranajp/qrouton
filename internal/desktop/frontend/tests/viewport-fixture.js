import { createViewportController } from "../src/lib/panes/viewport.js";

const root = document.querySelector("#root");
const content = document.querySelector("#content");
window.reports = [];
window.controller = createViewportController({
  root,
  content,
  selected: true,
  report: (value) => window.reports.push(value),
});
window.measure = () => window.controller.schedule();
window.setSelected = (value) => window.controller.setSelected(value);
