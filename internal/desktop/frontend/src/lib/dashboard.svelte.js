import { DASHBOARD_EVENT, DASHBOARD_REFRESH, DASHBOARD_SNAPSHOT } from "./bridge/generated.js";
import { call, Call, follow } from "./wails.js";

const NEVER = "0001-01-01T00:00:00Z";
const NOTHING = { viewer: "", checkedAt: NEVER, refreshing: false, failed: false, signedOut: false, pausedUntil: NEVER, sessions: {} };

/** dashboard is the pull request cache for every session, shared by every view of it. */
export function dashboard() {
  return (shared ??= observing());
}

let shared;

function observing() {
  let status = $state(/** @type {import("./dashboard/dashboard.js").DashboardStatus} */ (NOTHING));
  follow(DASHBOARD_EVENT, () => call(Call.ByName(DASHBOARD_SNAPSHOT)), (value) => {
    status = value ?? NOTHING;
  });
  return {
    get status() {
      return status;
    },
    refresh: () => call(Call.ByName(DASHBOARD_REFRESH)),
  };
}
