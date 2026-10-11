import assert from "node:assert/strict";
import test from "node:test";
import { agentName, buildRuns, elapsed, stateIcon, stateTone, stateWords, tokenLabel, toolLine, typeLabel, nowLine, overview, rootWords, sessionPath, stageName, stageTone, wroteLine } from "./tree.js";

/** @returns {any} */
const node = (id, parent, extra = {}) => ({
  id,
  run: "r1",
  parent,
  root: false,
  placed: true,
  type: id,
  state: "working",
  started: "2026-09-01T12:00:00Z",
  ...extra,
});
const root = (extra = {}) => node("root", "", { root: true, ...extra });
const ids = (branches) => branches.map((branch) => branch.node.id);

test("nodes nest to depth four under the root", () => {
  const nodes = [
    node("d", "c"),
    node("c", "b"),
    node("b", "a"),
    node("a", "root"),
    root(),
  ];
  const [run] = buildRuns(nodes);
  assert.equal(run.root.id, "root");
  const a = run.children[0];
  const b = a.children[0];
  const c = b.children[0];
  assert.deepEqual([a.node.id, b.node.id, c.node.id, c.children[0].node.id], ["a", "b", "c", "d"]);
});

test("a missing parent or a cycle falls back to the root", () => {
  const nodes = [root(), node("lost", "ghost"), node("x", "y"), node("y", "x"), node("self", "self")];
  const [run] = buildRuns(nodes);
  const attached = [];
  const walk = (branches) => branches.forEach((branch) => (attached.push(branch.node.id), walk(branch.children)));
  walk(run.children);
  assert.deepEqual(attached.sort(), ["lost", "self", "x", "y"]);
  assert.ok(ids(run.children).includes("lost"));
  assert.ok(ids(run.children).includes("self"));
});

test("two runs: the live one leads, else the latest root start", () => {
  const old = (id, extra = {}) => node(id, id === "r0" ? "" : "r0", { run: "old", ...extra });
  const nodes = [
    old("r0", { root: true, state: "ended", started: "2026-09-01T10:00:00Z" }),
    old("o1", { state: "ended" }),
    root(),
    node("a", "root"),
  ];
  assert.deepEqual(buildRuns(nodes).map((run) => run.run), ["r1", "old"]);

  const finished = [
    root({ state: "done", started: "2026-09-01T09:00:00Z" }),
    old("r0", { root: true, state: "done", started: "2026-09-01T10:00:00Z" }),
  ];
  assert.deepEqual(buildRuns(finished).map((run) => run.run), ["old", "r1"]);
});

test("live children come first, then by start time", () => {
  const nodes = [
    root(),
    node("done-early", "root", { state: "done", started: "2026-09-01T11:00:00Z" }),
    node("late", "root", { started: "2026-09-01T12:05:00Z" }),
    node("early", "root", { started: "2026-09-01T12:01:00Z" }),
    node("failed", "root", { state: "failed", started: "2026-09-01T10:00:00Z" }),
  ];
  assert.deepEqual(ids(buildRuns(nodes)[0].children), ["early", "late", "failed", "done-early"]);
});

test("working says thinking until a tool is in hand", () => {
  assert.equal(stateWords({ state: "working", tool: "" }), "thinking");
  assert.equal(stateWords({ state: "working", tool: "Bash" }), "working");
  assert.equal(stateWords({ state: "waiting_agents" }), "waiting on agents");
  assert.equal(stateWords({ state: "needs_you" }), "needs you");
  assert.equal(stateWords({ state: "your_turn" }), "your turn");
  assert.equal(stateWords({ state: "ended" }), "ended with run");
  assert.equal(stateTone("working"), "--state-running");
  assert.equal(stateTone("needs_you"), "--state-waiting");
  assert.equal(stateTone("your_turn"), "--accent-action");
  assert.equal(stateTone("failed"), "--state-failed");
  assert.equal(stateTone("idle"), "--text-faint");
});

test("labels", () => {
  const now = Date.parse("2026-09-01T12:00:00Z");
  assert.equal(elapsed("2026-09-01T11:59:18Z", "", now), "42s");
  assert.equal(elapsed("2026-09-01T11:56:55Z", "", now), "3m 5s");
  assert.equal(elapsed("2026-09-01T10:58:00Z", "", now), "1h 2m");
  assert.equal(elapsed("0001-01-01T00:00:00Z", "", now), "");
  assert.equal(elapsed("2026-09-01T11:00:00Z", "2026-09-01T11:05:00Z", now), "5m");
  assert.equal(tokenLabel(0), "");
  assert.equal(tokenLabel(850), "850 tokens");
  assert.equal(tokenLabel(42191), "42.2k tokens");
  assert.equal(typeLabel("qrouton-planning-lead"), "Qrouton Planning Lead");
});

test("a tool and its target read as two words", () => {
  assert.equal(toolLine({ tool: "Bash", target: "sleep 20" }), "Bash sleep 20");
  assert.equal(toolLine({ tool: "Bash", target: "" }), "Bash");
  assert.equal(toolLine({}), "");
});

test("only a runner reporting its tool calls can be thinking", () => {
  assert.equal(stateWords({ state: "working", tool: "" }, true), "thinking");
  assert.equal(stateWords({ state: "working", tool: "" }, false), "working");
  assert.equal(stateWords({ state: "working", tool: "Bash" }, false), "working");
});

const lead = (id, stage, started, state = "done", run = "1") => ({
  id, run, parent: "root", root: false, placed: true, type: "", stage, description: "", state, reason: "",
  tool: "", target: "", started, finished: "", toolCount: 0, tokens: 0, lastMessage: "", files: [], tools: [],
});

test("the overview lists leads in stage order, then by start, across runs", () => {
  const root = { ...lead("root", "", "2026-09-01T12:00:00Z", "waiting_agents"), root: true, parent: "" };
  const { leads } = overview([
    root,
    lead("i1", "implement", "2026-09-01T12:30:00Z", "working"),
    lead("x1", "", "2026-09-01T12:01:00Z"),
    lead("p1", "plan", "2026-09-01T12:20:00Z"),
    lead("r2", "research", "2026-09-01T12:10:00Z"),
    lead("r1", "research", "2026-09-01T11:00:00Z", "done", "0"),
  ]);
  assert.deepEqual(leads.map((branch) => branch.node.id), ["r1", "r2", "p1", "i1", "x1"]);
  assert.equal(rootWords(root, leads), "waiting on Implementation");
  assert.equal(stageName("plan"), "Plan");
  assert.equal(stageTone("implement"), "IMPLEMENT");
  assert.equal(stageTone(""), undefined);
});

test("paths inside the session read from the repository down", () => {
  assert.equal(sessionPath("/s/x/src/web/a.ts", "/s/x"), "web/a.ts");
  assert.equal(sessionPath("/s/x/thoughts/p.md", "/s/x"), "thoughts/p.md");
  assert.equal(sessionPath("/elsewhere/a.ts", "/s/x"), "/elsewhere/a.ts");
  assert.equal(nowLine({ tool: "Edit", target: "/s/x/src/web/pay.ts" }, "/s/x"), "Edit web/pay.ts");
  assert.equal(wroteLine(["/s/x/src/web/intent.ts", "/s/x/src/pay/stripe.go"]), "intent.ts, stripe.go");
});

test("an agent with no type is named by what it was spawned to do", () => {
  assert.equal(agentName({ type: "code-reviewer", description: "Review" }), "Code Reviewer");
  assert.equal(agentName({ type: "", description: "Run echo one" }), "Run echo one");
  assert.equal(agentName({ type: "", description: "" }), "Agent");
});

test("a state names the icon its row draws, and idle draws none", () => {
  assert.equal(stateIcon("working"), "running");
  assert.equal(stateIcon("waiting_agents"), "running");
  assert.equal(stateIcon("needs_you"), "waiting");
  assert.equal(stateIcon("failed"), "failed");
  assert.equal(stateIcon("done"), "succeeded");
  assert.equal(stateIcon("ended"), "succeeded");
  assert.equal(stateIcon("idle"), "");
  assert.equal(stateIcon("your_turn"), "");
});
