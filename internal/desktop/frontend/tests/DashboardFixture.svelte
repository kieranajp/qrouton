<script>
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
      repos: [{ name: "acme/web" }, { name: "acme/billing" }, { name: "acme/a-very-long-repository-name-that-overflows" }],
      opened: hours(21 * 24),
    }),
    ...Array.from({ length: 10 }, (_, i) => session(i + 2)),
  ]);
  let width = $state(1180);

  window.dashboard = {
    setSessions: (next) => (sessions = next),
    sessions: () => $state.snapshot(sessions),
    setWidth: (next) => (width = next),
  };
</script>

<div class="frame" style:width="{width}px">
  <Dashboard {sessions} />
</div>

<style>
  .frame {
    height: 100%;
    display: flex;
  }
</style>
