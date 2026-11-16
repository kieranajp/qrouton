import assert from "node:assert/strict";
import test from "node:test";
import { SIGN_IN_STATES, TAB_STATUSES, WINDOW_EXIT_EVENT, WINDOWS_EVENT } from "../bridge/generated.js";
import { badge, byProfile, followSignIn, pinnedNote, statusKey } from "./profiles.js";

test("a signed-in Claude badge names the account and a Codex one does not", () => {
  assert.deepEqual(badge({ runner: "claude", profile: "work", state: SIGN_IN_STATES.SIGNED_IN, email: "k@example.com" }), {
    text: "Signed in as k@example.com",
    tone: "success",
  });
  assert.deepEqual(badge({ runner: "codex", profile: "work", state: SIGN_IN_STATES.SIGNED_IN }), {
    text: "Signed in",
    tone: "success",
  });
});

test("a failed probe reads Unknown, never signed out", () => {
  assert.equal(badge({ runner: "claude", profile: "", state: SIGN_IN_STATES.UNKNOWN }).text, "Unknown");
  assert.equal(badge({ runner: "claude", profile: "", state: SIGN_IN_STATES.SIGNED_OUT }).text, "Signed out");
  assert.equal(badge(undefined).text, "Checking…");
});

test("the default profile and a named one keep separate badges", () => {
  const statuses = byProfile([
    { runner: "claude", profile: "", state: SIGN_IN_STATES.SIGNED_OUT },
    { runner: "claude", profile: "work", state: SIGN_IN_STATES.SIGNED_IN },
  ]);
  assert.equal(statuses[statusKey("claude", "")].state, SIGN_IN_STATES.SIGNED_OUT);
  assert.equal(statuses[statusKey("claude", "work")].state, SIGN_IN_STATES.SIGNED_IN);
});

/** A stand-in for the workbench's events: on subscribes, emit delivers a payload. */
function events() {
  /** @type {Map<string, Set<(data: any) => void>>} */
  const listeners = new Map();
  return {
    /** @param {string} event @param {(data: any) => void} listener */
    on: (event, listener) => {
      const set = listeners.get(event) ?? new Set();
      set.add(listener);
      listeners.set(event, set);
      return () => set.delete(listener);
    },
    /** @param {string} event @param {any} data */
    emit: (event, data) => [...(listeners.get(event) ?? [])].forEach((listener) => listener(data)),
    count: () => [...listeners.values()].reduce((total, set) => total + set.size, 0),
  };
}

/** @param {string} id */
const answered = (id) => () => Promise.resolve(/** @type {const} */ ({ ok: true, value: id }));

test("a sign-in re-probes when its own tab exits, once, and not for another tab", async () => {
  const bus = events();
  let probes = 0;
  await followSignIn(answered("window-7"), bus.on, () => probes++).started;

  bus.emit(WINDOW_EXIT_EVENT + "window-8", 0);
  assert.equal(probes, 0);
  bus.emit(WINDOW_EXIT_EVENT + "window-7", 0);
  bus.emit(WINDOW_EXIT_EVENT + "window-7", 0);
  assert.equal(probes, 1);
  assert.equal(bus.count(), 0, "a listener outlived the sign-in it was following");
});

test("a login that exits before Sign in answers still ends the sign-in", async () => {
  const bus = events();
  let probes = 0;
  /** @type {(value: any) => void} */
  let answer = () => {};
  const sign = followSignIn(() => new Promise((resolve) => (answer = resolve)), bus.on, () => probes++);

  bus.emit(WINDOWS_EVENT, { tabs: [{ id: "window-7", status: TAB_STATUSES.FAILED }] });
  answer({ ok: true, value: "window-7" });
  await sign.started;
  assert.equal(probes, 1);
  assert.equal(bus.count(), 0);
});

test("a tab still running when Sign in answers waits for its exit", async () => {
  const bus = events();
  let probes = 0;
  /** @type {(value: any) => void} */
  let answer = () => {};
  const sign = followSignIn(() => new Promise((resolve) => (answer = resolve)), bus.on, () => probes++);

  bus.emit(WINDOWS_EVENT, { tabs: [{ id: "window-7", status: TAB_STATUSES.RUNNING }] });
  answer({ ok: true, value: "window-7" });
  await sign.started;
  assert.equal(probes, 0);
  bus.emit(WINDOWS_EVENT, { tabs: [{ id: "window-7", status: TAB_STATUSES.SUCCEEDED }] });
  assert.equal(probes, 1);
});

test("a sign-in that could not start never reports an end and stops listening", async () => {
  const bus = events();
  let probes = 0;
  const answer = await followSignIn(() => Promise.resolve({ ok: false, error: "no session" }), bus.on, () => probes++)
    .started;
  assert.equal(answer.ok, false);
  assert.equal(probes, 0);
  assert.equal(bus.count(), 0);
});

test("a deleted profile's note shows the id that brings its sessions back", () => {
  assert.equal(
    pinnedNote({ id: "work", sessions: 2 }),
    "2 sessions are pinned to it and won't start until a profile with id “work” and this folder is added again.",
  );
  assert.equal(pinnedNote({ id: "work", sessions: 0 }), "No sessions use it.");
});

test("a pinned count that could not be read is not reported as none", () => {
  const note = pinnedNote({ id: "work", sessions: 0, uncounted: true });
  assert.match(note, /couldn't count/);
  assert.doesNotMatch(note, /No sessions/);
});
