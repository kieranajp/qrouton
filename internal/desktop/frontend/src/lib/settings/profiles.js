import { SIGN_IN_STATES, TAB_STATUSES, WINDOW_EXIT_EVENT, WINDOWS_EVENT } from "../bridge/generated.js";

/** @typedef {import("./calls.js").ProfileStatus} ProfileStatus */

/** The default profile's id is empty.
 * @param {string} runner @param {string} profile */
export const statusKey = (runner, profile) => `${runner}/${profile}`;

/** @param {ProfileStatus[] | undefined} statuses @returns {Record<string, ProfileStatus>} */
export const byProfile = (statuses) =>
  Object.fromEntries((statuses ?? []).map((status) => [statusKey(status.runner, status.profile), status]));

/** Undefined is a probe still running.
 * @param {ProfileStatus | undefined} status
 * @returns {{text: string, tone: "success" | "waiting" | "faint"}} */
export function badge(status) {
  if (!status) return { text: "Checking…", tone: "faint" };
  if (status.state === SIGN_IN_STATES.SIGNED_IN)
    return { text: status.email ? `Signed in as ${status.email}` : "Signed in", tone: "success" };
  if (status.state === SIGN_IN_STATES.SIGNED_OUT) return { text: "Signed out", tone: "waiting" };
  return { text: "Unknown", tone: "faint" };
}

/** @type {Set<string>} */
const FINISHED = new Set([TAB_STATUSES.SUCCEEDED, TAB_STATUSES.FAILED]);

/** Listens for the tab list before start runs, since a login can exit before its tab id is known.
 * @param {() => Promise<import("../wails.js").Answer<string>>} start @param {() => void} done
 * @param {(event: string, listener: (data: any) => void) => () => void} on */
export function followSignIn(start, on, done) {
  /** @type {string | undefined} */
  let tab;
  let over = false;
  /** @type {Set<string>} */
  const finished = new Set();
  let offExit = () => {};
  const stop = () => {
    over = true;
    offTabs();
    offExit();
  };
  const end = () => {
    if (over) return;
    stop();
    done();
  };
  const offTabs = on(WINDOWS_EVENT, (/** @type {{tabs?: {id: string, status?: string}[]}} */ surfaces) => {
    for (const item of surfaces?.tabs ?? []) if (FINISHED.has(item.status ?? "")) finished.add(item.id);
    if (tab !== undefined && finished.has(tab)) end();
  });
  const started = start().then((answer) => {
    if (!answer.ok) {
      stop();
      return answer;
    }
    tab = answer.value;
    if (!over) offExit = on(WINDOW_EXIT_EVENT + tab, end);
    if (finished.has(tab)) end();
    return answer;
  });
  return { started, stop };
}

/** @param {{id: string, sessions: number, uncounted?: boolean}} profile */
export function pinnedNote(profile) {
  const back = `won't start until a profile with id “${profile.id}” and this folder is added again.`;
  if (profile.uncounted) return `qrouton couldn't count the sessions using it. Any pinned to it ${back}`;
  if (profile.sessions === 0) return "No sessions use it.";
  return `${profile.sessions === 1 ? "1 session is" : `${profile.sessions} sessions are`} pinned to it and ${back}`;
}
