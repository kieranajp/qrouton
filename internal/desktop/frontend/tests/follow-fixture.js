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
    const settle = options.settle ? (answer, live) => applied.push({ settled: answer.value, live }) : undefined;
    window.following = follow(EVENT, snapshot, (value) => applied.push(value ?? null), { accepts, settle });
  },
  emit: (data) => emitWailsEvent(EVENT, data),
  answer: (value) => answer(value),
  stop: () => window.following.stop(),
  refresh: () => window.following.refresh(),
  applied: () => applied,
};
