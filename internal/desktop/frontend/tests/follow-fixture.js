import { follow } from "../src/lib/wails.js";
import { emitWailsEvent } from "./wails-runtime.js";

const EVENT = "follow:test";
let applied = [];
let answer;

window.followFixture = {
  start(options = {}) {
    applied = [];
    const snapshot = () => new Promise((resolve) => (answer = resolve));
    const accepts = options.only ? (data) => data?.only === options.only : undefined;
    window.following = follow(EVENT, snapshot, (value) => applied.push(value ?? null), { accepts });
  },
  emit: (data) => emitWailsEvent(EVENT, data),
  answer: (value) => answer(value),
  stop: () => window.following.stop(),
  applied: () => applied,
};
