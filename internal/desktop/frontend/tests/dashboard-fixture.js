import "../src/tokens/index.css";
import { mount } from "svelte";
import { DASHBOARD_EVENT, DASHBOARD_SNAPSHOT, SESSIONS_SHOW } from "../src/lib/bridge/generated.js";
import { emitWailsEvent } from "./wails-runtime.js";
import DashboardFixture from "./DashboardFixture.svelte";

const NEVER = "0001-01-01T00:00:00Z";
// Half a minute past the mark, so a page clock read a moment earlier still floors to the same minute.
const minutesAgo = (minutes) => new Date(Date.now() - (minutes + 0.5) * 60000).toISOString();
const pull = (repo, number, title, extra = {}) => ({
  repo,
  number,
  title,
  url: `https://github.com/${repo}/pull/${number}`,
  state: "open",
  author: "octocat",
  review: "",
  checksFailing: false,
  updatedAt: minutesAgo(number),
  ...extra,
});
const stat = (name, commits, insertions, deletions, role = "editing") => ({ name, role, path: "", commits, insertions, deletions, measured: true });

const status = (extra = {}) => ({
  viewer: "octocat",
  checkedAt: minutesAgo(10),
  refreshing: false,
  failed: false,
  signedOut: false,
  pausedUntil: NEVER,
  sessions: {
    ...Object.fromEntries(Array.from({ length: 10 }, (_, i) => [`session-${i + 2}`, { pullRequests: [], pullState: "none", repos: [] }])),
    billing: {
      pullState: "answered",
      pullRequests: [
        pull("acme/billing", 3, "Retire the v1 billing client", { state: "merged", review: "approved" }),
        pull("acme/web", 41, "Swap PaymentIntent for the v2 client and move refunds onto the new webhook handler", {
          review: "changes-requested",
          checksFailing: true,
        }),
        pull("acme/web", 7, "Their pull request on the same branch", { author: "someone-else" }),
      ],
      repos: [stat("acme/web", 3, 600, 200), stat("acme/billing", 2, 11, 4), stat("acme/docs", 9, 9, 9, "reference")],
    },
    "session-3": {
      pullState: "answered",
      pullRequests: Array.from({ length: 7 }, (_, i) => pull("acme/api", i + 1, `Endpoint ${i + 1}`)),
      repos: [stat("acme/api", 0, 0, 0)],
    },
  },
  ...extra,
});

const calls = [];
const opened = [];
const settings = [];

let refuseShow = false;

window.dashboardBridge = { calls: () => [...calls], opened, settings, refuseNextShow: () => (refuseShow = true) };
window.dashboardStatus = status;
window.pushDashboard = (next) => emitWailsEvent(DASHBOARD_EVENT, next);

window.wailsCall = async (name, ...args) => {
  if (name === DASHBOARD_SNAPSHOT) return status();
  calls.push({ method: name.split(".").at(-1), args });
  if (name === SESSIONS_SHOW && refuseShow) {
    refuseShow = false;
    throw new Error("session is not loaded");
  }
};

mount(DashboardFixture, { target: document.querySelector("#fixture") });
