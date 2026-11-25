<script>
  import { dashboard } from "../src/lib/dashboard.svelte.js";
  import Dashboard from "../src/lib/dashboard/Dashboard.svelte";
  import { openTodo } from "../src/lib/overview/calls.js";
  import { show } from "../src/lib/sessions.js";
  import { call } from "../src/lib/wails.js";

  const hours = (count) => new Date(Date.now() - count * 3600000).toISOString();
  // Opened most recently further down the rail, so a sort by last opened would reverse the cards.
  const session = (index, extra = {}) => ({
    name: `Session ${index}`,
    slug: `session-${index}`,
    mode: "ASSISTANT",
    repos: [],
    opened: hours(index),
    ...extra,
  });

  let sessions = $state([
    session(1, {
      name: "Extract the billing service into its own repository so the checkout team can own it",
      slug: "billing",
      mode: "RPI",
      repos: [
        { name: "acme/web", role: "editing" },
        { name: "acme/billing", role: "editing" },
        { name: "acme/a-very-long-repository-name-that-overflows", role: "editing" },
      ],
      opened: hours(21 * 24),
    }),
    session(2, {
      todos: {
        open: 3,
        items: [
          { text: "Apply the migration", section: "Before merge", where: "", path: "thoughts/shared/todos/T1-rollout.md", line: 14 },
        ],
      },
    }),
    ...Array.from({ length: 9 }, (_, i) => session(i + 3, { todos: { open: 0, items: [] } })),
  ]);
  let width = $state(1180);
  let shown = $state(true);

  window.dashboard = {
    setSessions: (next) => (sessions = next),
    sessions: () => $state.snapshot(sessions),
    setWidth: (next) => (width = next),
    setShown: (next) => (shown = next),
  };
</script>

<div class="frame" style:width="{width}px">
  {#if shown}
    <Dashboard
      {sessions}
      onOpen={(slug) => window.dashboardBridge.opened.push(slug)}
      onOpenTodo={(slug, path, line) => openTodo(async (shown) => (await call(show(shown))).ok, slug, path, line)}
      store={dashboard()}
      onSettings={(tab) => window.dashboardBridge.settings.push(tab)} />
  {/if}
</div>

<style>
  .frame {
    height: 100%;
    display: flex;
  }
</style>
