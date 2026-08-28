<script>
  import Rail from "../src/lib/session/Rail.svelte";

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
    removeFinished: () => {
      agents = { ...agents, agents: agents.agents.filter((agent) => agent.state !== "Finished") };
    },
    rootOnly,
    added: () => added,
  };
</script>

<div class="frame">
  <Rail
    {sessions}
    slug="checkout"
    {repos}
    {agents}
    onNewSession={() => {}}
    onAddRepos={() => added++}
    onDismissed={() => {}} />
</div>

<style>
  .frame {
    height: 100%;
    display: flex;
  }
</style>
