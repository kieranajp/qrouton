<script>
  const TONES = {
    neutral: "var(--text-secondary)",
    editing: "var(--role-editing)",
    reference: "var(--role-reference)",
    guided: "var(--state-guided)",
    assistant: "var(--state-running)",
    running: "var(--state-running)",
    success: "var(--state-success)",
    waiting: "var(--state-waiting)",
    failed: "var(--state-failed)",
  };

  /** @type {{tone?: string, selected?: boolean, glyph?: string, meta?: string, children?: import('svelte').Snippet, [attribute: string]: any}} */
  let { tone = "neutral", selected = false, glyph, meta, children, ...rest } = $props();
</script>

<span class="chip" class:selected style:--tone={TONES[tone]} {...rest}>
  {#if glyph}<span aria-hidden="true">{glyph}</span>{/if}
  {@render children?.()}
  {#if meta}<span class="meta">{meta}</span>{/if}
</span>

<style>
  .chip {
    display: inline-flex;
    gap: 6px;
    font: var(--machine-sm);
    font-size: 11px;
    color: var(--tone);
    background: var(--surface-chrome);
    border: 1px solid var(--border-subtle);
    padding: 2px 7px;
  }

  .selected {
    font: var(--machine-bold);
    font-size: 11px;
    color: var(--text-on-accent);
    background: var(--tone);
    border: none;
    padding: 3px 9px;
  }

  .meta {
    color: var(--text-faint);
  }
</style>
