import "../src/tokens/scale.css";
import "../src/tokens/typography.css";
import "../src/tokens/spacing.css";
import "../src/tokens/effects.css";
import { mount } from "svelte";
import { AGENT_TREE_EVENT, CHROME_AGENT_TREE, WINDOWS_OPEN_DOCUMENT } from "../src/lib/bridge/generated.js";
import { emitWailsEvent } from "./wails-runtime.js";
import OverviewFixture from "./OverviewFixture.svelte";

const NEVER = "0001-01-01T00:00:00Z";
const node = (id, parent, extra = {}) => ({
  id,
  run: "run-1",
  parent,
  root: false,
  placed: true,
  type: "",
  stage: "",
  description: "",
  state: "working",
  reason: "",
  tool: "",
  target: "",
  started: "2026-09-01T12:00:00Z",
  finished: NEVER,
  toolCount: 0,
  tokens: 0,
  lastMessage: "",
  files: [],
  documents: [],
  tools: [],
  ...extra,
});
const done = (finished) => ({ state: "done", finished });

// The design's moment: Research and Plan are done, the Implementation lead
// has three subagents working and one finished.
const session = {
  provider: "claude",
  providerLabel: "Claude Code",
  coverage: "",
  tools: true,
  nodes: [
    node("root", "", { root: true, state: "waiting_agents" }),
    node("research", "root", {
      type: "qrouton-research-lead", stage: "research", ...done("2026-09-01T12:05:02Z"),
      documents: [{ name: "R3-retry.md", path: "thoughts/shared/research/R3-retry.md", kind: "RESEARCH", id: "R3" }],
    }),
    node("r-sub-1", "research", { type: "codebase-researcher", ...done("2026-09-01T12:03:00Z") }),
    node("r-sub-2", "research", { type: "pattern-finder", ...done("2026-09-01T12:04:00Z") }),
    node("plan", "root", {
      type: "qrouton-planning-lead", stage: "plan", started: "2026-09-01T12:06:00Z", ...done("2026-09-01T12:08:48Z"),
      documents: [{ name: "P2-checkout.md", path: "thoughts/shared/plans/P2-checkout.md", kind: "PLAN", id: "P2" }],
    }),
    node("implement", "root", {
      type: "qrouton-implementation-lead", stage: "implement", started: "2026-09-01T12:10:00Z", tool: "Agent",
      target: "Partial-capture refunds",
    }),
    node("impl-1", "implement", {
      type: "implementer", description: "Swap PaymentIntent for the v2 client", started: "2026-09-01T12:11:00Z",
      tool: "Edit", target: "/s/checkout/src/web/checkout/pay.ts",
      files: ["/s/checkout/src/web/checkout/intent.ts", "/s/checkout/src/payments/stripe.go"],
      tools: [{ tool: "Edit", target: "/s/checkout/src/web/checkout/intent.ts", start: "2026-09-01T12:11:10Z", end: "2026-09-01T12:11:12Z", failed: false }],
    }),
    node("review-1", "implement", {
      type: "code-reviewer", description: "Phase 1 against § Verify", started: "2026-09-01T12:12:00Z",
      tool: "Read", target: "/s/checkout/src/web/checkout/intent.ts",
    }),
    node("impl-2", "implement", { type: "implementer", description: "Partial-capture refunds", started: "2026-09-01T12:13:00Z" }),
    node("tests-1", "implement", {
      type: "test-writer", started: "2026-09-01T12:10:30Z", ...done("2026-09-01T12:13:10Z"),
      files: ["/s/checkout/src/web/checkout/refund.test.ts"], tokens: 18400, lastMessage: "Wrote the refund tests.",
    }),
  ],
};

const opencode = {
  provider: "opencode",
  providerLabel: "OpenCode",
  coverage: "OpenCode reports the root agent only, so working and idle come from its terminal.",
  tools: false,
  nodes: [node("root", "", { root: true, state: "working" })],
};
const codex = {
  provider: "codex",
  providerLabel: "Codex",
  coverage: "",
  tools: false,
  nodes: [
    node("codex-root", "", { root: true, state: "waiting_agents" }),
    node("codex-direct-a", "codex-root", {
      type: "codebase-researcher", taskName: "/root/review_security", started: "2026-09-01T12:01:00Z",
    }),
    node("codex-direct-b", "codex-root", {
      type: "codebase-researcher",
      taskName: "/root/a_very_long_assigned_name_for_security_research_that_should_truncate_in_the_row",
      started: "2026-09-01T12:02:00Z",
    }),
    node("codex-direct-unnamed", "codex-root", {
      type: "codebase-researcher", started: "2026-09-01T12:03:00Z",
    }),
    node("codex-direct-finished", "codex-root", {
      type: "codebase-researcher", taskName: "/root/finished_research", started: "2026-09-01T12:04:00Z",
      ...done("2026-09-01T12:05:00Z"),
    }),
    node("codex-parent", "codex-direct-a", { type: "implementer", taskName: "/root/review_security/child_work" }),
    node("codex-nested-named", "codex-parent", {
      type: "code-reviewer", taskName: "/root/review_security/child_work/check_authentication_and_authorization_for_every_privileged_request_and_token_refresh_case", started: "2026-09-01T12:06:00Z",
    }),
    node("codex-nested-unnamed", "codex-parent", {
      type: "code-reviewer", started: "2026-09-01T12:07:00Z",
    }),
    node("codex-nested-finished", "codex-parent", {
      type: "code-reviewer", taskName: "/root/review_security/child_work/write_tests", started: "2026-09-01T12:08:00Z",
      ...done("2026-09-01T12:09:00Z"),
    }),
  ],
};

const enriched = {
  ...codex,
  nodes: codex.nodes.map((entry) => entry.id === "codex-direct-unnamed" ? { ...entry, taskName: "/root/late_name" } : entry),
};

const trees = { default: session, opencode, codex, enriched };
const params = new URLSearchParams(location.search);
const tree = trees[params.get("tree") ?? "default"];
if (params.has("longdoing")) {
  const root = tree.nodes.find((n) => n.root);
  root.tool = "Bash";
  root.target = "go test ./internal/desktop/... -run TestAgentTree -count=1 -v " + "./some/very/long/path/".repeat(12);
}
const minutesAgo = (minutes) => new Date(Date.now() - minutes * 60000).toISOString();
const summaries = {
  long: {
    text: "Migrating checkout to the PaymentIntent v2 client, with partial-capture refunds and webhook parity in acme/payments. Docs stay read-only.",
    stage: "implement",
    at: minutesAgo(4),
  },
  short: { text: "Fix flaky refund webhook test.", stage: "plan", at: minutesAgo(1) },
  empty: { text: "", stage: "", at: "" },
};
const summary = summaries[params.get("summary") ?? "long"];
if (params.has("narrow")) document.querySelector("#fixture").classList.add("narrow");

window.opened = [];
window.wailsCall = async (name, ...args) => {
  if (name === CHROME_AGENT_TREE) return tree;
  if (name === WINDOWS_OPEN_DOCUMENT) window.opened.push(args[0]);
  return undefined;
};
window.pushTree = (key) => emitWailsEvent(AGENT_TREE_EVENT + "fixture", trees[key]);
window.trees = trees;

mount(OverviewFixture, { target: document.querySelector("#fixture"), props: { summary } });
