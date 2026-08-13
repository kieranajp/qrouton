import { createTerminalActivation } from "../src/lib/terminal-focus.js";

const conversation = document.querySelector("#conversation");
const pane = document.querySelector("#terminal-pane");
const terminal = document.querySelector("#terminal");
const userSelect = document.querySelector("#user-select");

let generation = 0;
let pending = false;
window.refits = 0;
window.focuses = 0;
terminal.addEventListener("focus", () => window.focuses++);

const mount = () =>
  createTerminalActivation({
    frame: requestAnimationFrame,
    cancelFrame: cancelAnimationFrame,
    refit: () => window.refits++,
    focus: () => terminal.focus(),
    handled: (handled) => {
      if (handled === generation) pending = false;
    },
  });

let activation = mount();

window.agentSelect = () => {
  pane.hidden = false;
  activation.update(true, generation, pending);
};

window.userSelectBeforeMount = () => {
  activation.destroy();
  generation++;
  pending = true;
  activation = mount();
  pane.hidden = false;
  activation.update(true, generation, pending);
};

window.remount = () => {
  activation.destroy();
  activation = mount();
  activation.update(true, generation, pending);
};

userSelect.addEventListener("click", () => {
  pane.hidden = false;
  generation++;
  pending = true;
  activation.update(true, generation, pending);
});

conversation.focus();
