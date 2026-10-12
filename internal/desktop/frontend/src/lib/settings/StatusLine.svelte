<script>
  const TONES = {
    success: "var(--state-success)",
    waiting: "var(--state-waiting)",
    failed: "var(--state-failed)",
    faint: "var(--text-faint)",
  };

  /** @type {{tone?: 'success'|'waiting'|'failed'|'faint', boxed?: boolean, detail?: string, children: import('svelte').Snippet, body?: import('svelte').Snippet, actions?: import('svelte').Snippet}} */
  let { tone = "faint", boxed = false, detail = "", children, body, actions } = $props();
</script>

<div class="status-line" class:boxed>
  <div class="sentence">
    <span class="dot" style:background={TONES[tone]}></span>
    <span class="text">{@render children()}</span>
  </div>
  {@render body?.()}
  {#if actions || detail}
    <div class="actions">
      {@render actions?.()}
      {#if detail}<span class="detail">{detail}</span>{/if}
    </div>
  {/if}
</div>

<style>
  .status-line {
    display: flex;
    flex-direction: column;
    gap: calc(12px * var(--ui-scale));
    max-width: calc(560px * var(--ui-scale));
  }

  .boxed {
    padding: calc(14px * var(--ui-scale)) calc(16px * var(--ui-scale));
    border: 1px solid var(--border-default);
    background: var(--surface-chrome);
  }

  .sentence {
    display: flex;
    align-items: center;
    gap: calc(8px * var(--ui-scale));
  }

  .dot {
    width: calc(7px * var(--ui-scale));
    height: calc(7px * var(--ui-scale));
    flex: none;
  }

  .text {
    font: var(--machine-sm);
    color: var(--text-secondary);
  }

  .boxed .text {
    color: var(--text-primary);
  }

  .actions {
    display: flex;
    align-items: center;
    gap: calc(12px * var(--ui-scale));
  }

  .detail {
    font: var(--machine-xs);
    color: var(--text-muted);
  }
</style>
