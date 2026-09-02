<script>
  import { onMount } from "svelte";
  import Rail from "../src/lib/session/Rail.svelte";
  import { CHROME_EVENT } from "../src/lib/bridge/generated.js";
  import { DEFAULT_STICKER_LABELS } from "../src/lib/session/stickers.js";
  import { Events } from "../src/lib/wails.js";

  const first = {
    name: "Checkout migration",
    slug: "checkout",
    initials: "CM",
    repos: [
      { name: "acme/web", role: "editing" },
      { name: "acme/a-very-long-editing-repository-name", role: "editing" },
    ],
    summary: { attention: "needs-you", active: 2, coverage: "full", running: true },
    unseen: 3,
    opened: new Date(Date.now() - 120000).toISOString(),
    sticker: "",
  };
  let sessions = $state([
    first,
    ...Array.from({ length: 11 }, (_, index) => ({
      name: `Session ${index + 2}`,
      slug: `session-${index + 2}`,
      initials: `S${index + 2}`,
      repos: [],
      summary: { attention: "none", active: 0, coverage: "none", running: false },
      unseen: 0,
      opened: new Date(Date.now() - (index + 1) * 86400000).toISOString(),
      sticker: "",
    })),
  ]);
  let agents = $state({
    provider: "claude",
    attention_known: true,
    children_known: true,
    parents_known: false,
    outcomes_known: false,
    agents: [
      {
        id: "root",
        run_id: "7",
        provider: "claude",
        role: "Orchestrator",
        state: "Waiting for you",
      },
      {
        id: "lead-1",
        run_id: "7",
        provider: "claude",
        parent_id: "root",
        parent_known: true,
        type: "qrspi-planning-lead",
        role: "Lead",
        state: "Active",
      },
      {
        id: "agent-flat",
        run_id: "7",
        provider: "claude",
        parent_id: "lead-1",
        parent_known: true,
        type: "code-reviewer",
        role: "Specialist",
        state: "Active",
      },
      {
        id: "agent-finished",
        run_id: "6",
        provider: "claude",
        parent_known: false,
        type: "test-verifier",
        role: "Specialist",
        state: "Finished",
      },
      ...Array.from({ length: 5 }, (_, index) => ({
        id: `agent-${index}`,
        run_id: "7",
        provider: "claude",
        parent_known: false,
        type: `specialist-${index}`,
        role: "Specialist",
        state: "Active",
      })),
    ],
  });
  const repos = [
    { name: "acme/web", role: "editing", measured: true, commits: 2 },
    { name: "acme/reference-docs", role: "reference", measured: true, commits: 0 },
    ...Array.from({ length: 7 }, (_, index) => ({
      name: `acme/repository-${index}`,
      role: "editing",
      measured: true,
      commits: index,
    })),
  ];
  let added = $state(0);
  let selected = $state("checkout");
  let stickerLabels = $state({ ...DEFAULT_STICKER_LABELS });

  onMount(() =>
    Events.On(CHROME_EVENT, (event) => {
      stickerLabels = { ...event.data.stickerLabels };
    }),
  );

  function rootOnly(provider) {
    const childrenKnown = provider === "codex";
    sessions = sessions.map((session) =>
      session.slug === "checkout"
        ? { ...session, summary: { attention: "unknown", active: 1, coverage: childrenKnown ? "full" : "root", running: true } }
        : session,
    );
    agents = {
      provider,
      attention_known: false,
      children_known: childrenKnown,
      parents_known: false,
      outcomes_known: false,
      agents: [
        {
          id: "root",
          run_id: "8",
          provider,
          role: "Orchestrator",
          state: "Working",
        },
      ],
    };
  }

  window.sessionRail = {
    quiet: () => {
      agents = { ...agents, agents: [] };
    },
    removeFinished: () => {
      agents = { ...agents, agents: agents.agents.filter((agent) => agent.state !== "Finished") };
    },
    rootOnly,
    added: () => added,
    shown: (slug) => (selected = slug),
    rename: (slug, name) => {
      sessions = sessions.map((session) => (session.slug === slug ? { ...session, name } : session));
    },
    stickerChanged: (slug, sticker) => {
      sessions = sessions.map((session) =>
        session.slug === slug ? { ...session, sticker } : session,
      );
    },
    setStickerLabels: (labels) => (stickerLabels = { ...stickerLabels, ...labels }),
  };
</script>

<button class="focus-probe" aria-label="Conversation focus">Conversation focus</button>
<div class="frame">
  <Rail
    {sessions}
    slug={selected}
    {repos}
    {agents}
    {stickerLabels}
    onNewSession={() => {}}
    onAddRepos={() => added++}
    onDismissed={() => {}} />
</div>

<style>
  .frame {
    height: 100%;
    display: flex;
  }

  .focus-probe {
    position: fixed;
    left: -10000px;
  }
</style>
