import "../src/tokens/typography.css";
import "../src/tokens/spacing.css";
import "../src/tokens/effects.css";
import { mount } from "svelte";
import Session from "../src/Session.svelte";
import { emitWailsEvent } from "./wails-runtime.js";

const calls = [];
const reports = new Map();
let slug = "first";
let delayed = false;
let delayedConfirm = false;
let failedConfirm = false;
let delayedCancel = false;
const loads = [];
const confirms = [];
const cancellations = [];
const body = "<img src=\"https://report.invalid/track\">\n![remote](https://report.invalid/image)\n" + Array.from({ length: 100 }, (_, i) => `Evidence line ${i + 1}`).join("\n") + "\nfinal evidence";
const fields = () => ({
  slug, identity: slug, mode: "RPI", terminal: "term-" + slug,
  sessions: ["first", "second"].map((s) => ({ slug: s, name: s, terminal: "term-" + s, repos: [] })),
  bugReportID: reports.get(slug)?.report.id ?? "",
  bugReportStatus: reports.get(slug)?.report.status ?? "",
  bugReportLabel: "Review bug report",
});
const emit = () => emitWailsEvent("chrome:update", fields());
const outcome = (owner, state) => {
  const preview = reports.get(owner);
  preview.report = {
    ...preview.report, status: state,
    message: state === "unknown" ? "An issue may have been created; inspect GitHub before retrying." :
      state === "created" ? "Created GitHub issue #42." :
      state === "posting" ? "Creating the confirmed GitHub issue…" :
      state === "failed" ? "No GitHub token: run gh auth login." :
      state === "expired" ? "Bug report review expired. No issue was sent." :
      "Bug report cancelled. No issue was sent.",
    ...(state === "created" ? { number: 42, url: "https://github.com/kieranajp/qrouton/issues/42" } : {}),
  };
  emit();
  return structuredClone(preview.report);
};

window.wailsCall = (name, ...args) => {
  calls.push({ name, args });
  if (name.endsWith("Chrome.Snapshot")) return fields();
  if (name.endsWith("Windows.Surfaces")) return { session: args[0], selected: "", tabs: [] };
  if (name.endsWith("BugReports.Load")) {
    const value = structuredClone(reports.get(args[0]));
    if (delayed) return new Promise((resolve) => loads.push(() => resolve(value)));
    return value;
  }
  if (name.endsWith("BugReports.Confirm")) {
    if (failedConfirm) return Promise.reject(new Error("connection lost"));
    const value = outcome(args[0], "posting");
    if (delayedConfirm) return new Promise((resolve) => confirms.push(() => resolve(value)));
    return value;
  }
  if (name.endsWith("BugReports.Cancel")) {
    const value = outcome(args[0], "cancelled");
    if (delayedCancel) return new Promise((resolve) => cancellations.push(() => resolve(value)));
    return value;
  }
  if (name.endsWith("Orgs.List") || name.endsWith("Repositories.Cached")) return [];
  return undefined;
};

window.bugs = {
  calls: (suffix) => calls.filter(({ name }) => name.endsWith(suffix)).map(({ args }) => args),
  queue: (owner = "first", id = "one") => {
    reports.set(owner, {
      title: "A complete bug title " + id, body, destination: "kieranajp/qrouton",
      reviewLabel: "Review bug report", createLabel: "Create issue", cancelLabel: "Cancel", closeLabel: "Close",
      unknownMessage: "An issue may have been created; inspect GitHub before retrying.",
      report: { id, status: "pending", message: "Review the bug report in qrouton before creating an issue." },
    });
    emit();
  },
  select: (next) => { slug = next; emit(); },
  finish: (state, owner = slug) => outcome(owner, state),
  delay: (value) => { delayed = value; },
  delayConfirm: () => { delayedConfirm = true; },
  failConfirm: () => { failedConfirm = true; },
  delayCancel: () => { delayedCancel = true; },
  releaseConfirm: () => confirms.splice(0).forEach((resolve) => resolve()),
  releaseCancel: () => cancellations.splice(0).forEach((resolve) => resolve()),
  release: () => { delayed = false; loads.splice(0).forEach((resolve) => resolve()); },
};

mount(Session, { target: document.querySelector("#fixture") });
