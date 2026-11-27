<script>
  import { dashboard } from "../src/lib/dashboard.svelte.js";
  import Dashboard from "../src/lib/dashboard/Dashboard.svelte";

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
    ...Array.from({ length: 10 }, (_, i) => session(i + 2)),
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
