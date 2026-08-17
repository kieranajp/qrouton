<script>
  import CapsLabel from "../core/CapsLabel.svelte";

  const PANELS = [
    {
      label: "Sessions",
      width: 70,
      background: "var(--surface-chrome)",
      border: "var(--border-subtle)",
      colour: "var(--text-faint)",
    },
    {
      label: "Agent",
      width: 230,
      background: "var(--surface-app)",
      border: "var(--accent-action)",
      colour: "var(--accent-label)",
    },
    {
      label: "Yours",
      width: 150,
      background: "var(--surface-terminal)",
      border: "var(--border-subtle)",
      colour: "var(--text-faint)",
    },
  ];
</script>

<h1>Three panels, one window</h1>

<div class="split">
  <div class="prose">
    <p>
      Your open sessions on the left. The agent in the middle. A terminal that is yours alone on the
      right — the agent never types into it.
    </p>
    <p>
      When the agent has something you should look at — a plan, a diff, a running command — it opens
      its own surface rather than burying it in the conversation.
    </p>
  </div>

  <div class="aside">
    <div class="panels">
      {#each PANELS as panel (panel.label)}
        <div
          class="panel"
          style:flex="{panel.width} 0 0"
          style:background={panel.background}
          style:border-color={panel.border}>
          <span class="label" style:color={panel.colour}>{panel.label}</span>
        </div>
      {/each}
    </div>

    <div class="note">
      <span class="swatch"></span>
      <span>◆ Plans, diffs and docs open as their own surfaces. Close one and nothing stops.</span>
    </div>
  </div>
</div>

<div class="foot">
  <CapsLabel tone="dim">Two ways the agent works</CapsLabel>
  <p>
    <span class="guided">Guided</span> researches and plans before it writes anything ·
    <span class="open">Open-ended</span> just helps. You choose per session, and can switch
    mid-conversation.
  </p>
</div>

<style>
  h1 {
    margin: 0;
    font: var(--display-md);
    letter-spacing: var(--display-tracking);
    color: var(--text-primary);
  }

  .split {
    display: flex;
    gap: 34px;
    align-items: flex-start;
  }

  .prose {
    flex: 1;
    display: flex;
    flex-direction: column;
    gap: 14px;
  }

  p {
    margin: 0;
    font: var(--machine-md);
    color: var(--text-secondary);
  }

  .aside {
    flex: 1;
    display: flex;
    flex-direction: column;
    gap: 14px;
  }

  .panels {
    display: flex;
    gap: 8px;
  }

  .panel {
    height: 96px;
    min-width: 0;
    display: flex;
    align-items: flex-start;
    padding: 10px;
    border: 1px solid;
  }

  .label {
    font: var(--instruction);
    letter-spacing: var(--instruction-tracking);
    text-transform: uppercase;
    white-space: nowrap;
    overflow: hidden;
  }

  .note {
    display: flex;
    align-items: baseline;
    gap: 10px;
    padding: 10px 12px;
    border: 1px solid var(--state-guided);
    font: var(--machine-sm);
    color: var(--text-secondary);
  }

  .swatch {
    width: 12px;
    height: 12px;
    flex: none;
    background: var(--state-guided);
  }

  .foot {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .foot p {
    font: var(--machine-sm);
  }

  .guided {
    color: var(--state-guided);
  }

  .open {
    color: var(--text-primary);
  }
</style>
