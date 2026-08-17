<script>
  const TREE = [
    { path: "~/work/qrouton/" },
    { path: "├── .mirrors/", note: "cloned once, shared" },
    { path: "└── extract-billing-service/", session: true },
    { path: "    ├── src/lifesum-api/", note: "editing", tone: "editing" },
    { path: "    ├── src/lifesum-billing/", note: "editing", tone: "editing" },
    { path: "    ├── src/lifesum-web/", note: "reference", tone: "reference" },
    { path: "    └── thoughts/", note: "what the agent writes down" },
  ];

  const LEGEND = [
    {
      swatch: "var(--role-editing)",
      lead: "Editing",
      rest: "— the agent may change these, on a fresh branch.",
    },
    {
      swatch: "var(--role-reference)",
      lead: "Reference",
      rest: "— checked out so the agent can read them, marked read-only.",
    },
  ];
</script>

<h1>A session is one piece of work</h1>

<div class="split">
  <div class="prose">
    <p>
      Not a project, not a repo — a task. "Extract the billing service" is a session. It owns a
      folder, a branch across every repo it touches, and its own agent conversation.
    </p>
    <p>Have as many open as you like. Come back to one and it picks up where the agent left off.</p>
  </div>

  <div class="aside">
    <div class="tree">
      {#each TREE as row (row.path)}
        <span class="path" class:session={row.session}>{row.path}</span>
        <span class="note {row.tone ?? ''}">{row.note ?? ""}</span>
      {/each}
      <span class="gap"></span>
      <span class="gap"></span>
      <span class="label">branch:</span>
      <span class="value">feat/extract-billing-service</span>
      <span class="label"></span>
      <span class="value">in every editing repo</span>
    </div>

    <div class="legend">
      {#each LEGEND as row (row.lead)}
        <div class="entry">
          <span class="swatch" style:background={row.swatch}></span>
          <span><span class="lead">{row.lead}</span> {row.rest}</span>
        </div>
      {/each}
    </div>
  </div>
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

  .tree {
    display: grid;
    grid-template-columns: max-content 1fr;
    column-gap: 14px;
    padding: 16px 18px;
    background: var(--surface-terminal);
    border: 1px solid var(--border-subtle);
    font: var(--terminal-sm);
    color: var(--text-secondary);
    white-space: pre;
  }

  .path.session {
    color: var(--accent-label);
  }

  .note {
    color: var(--text-faint);
  }

  .note.editing {
    color: var(--role-editing);
  }

  .note.reference {
    color: var(--role-reference);
  }

  .gap {
    height: 12px;
  }

  .label {
    color: var(--text-faint);
  }

  .value {
    color: var(--text-secondary);
  }

  .legend {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .entry {
    display: flex;
    align-items: baseline;
    gap: 10px;
    font: var(--machine-sm);
    color: var(--text-secondary);
  }

  .swatch {
    width: 13px;
    height: 13px;
    flex: none;
  }

  .lead {
    color: var(--text-primary);
  }
</style>
