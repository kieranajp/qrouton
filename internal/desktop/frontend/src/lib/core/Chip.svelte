<script>
  import { REPO_ROLES } from "../bridge/generated.js";
  import Icon from "./Icon.svelte";

  const TONES = {
    neutral: "var(--text-secondary)",
    [REPO_ROLES.EDITING]: "var(--role-editing)",
    [REPO_ROLES.REFERENCE]: "var(--role-reference)",
    guided: "var(--state-guided)",
    assistant: "var(--state-running)",
    running: "var(--state-running)",
    success: "var(--state-success)",
    waiting: "var(--state-waiting)",
    failed: "var(--state-failed)",
  };

  /** @type {{tone?: string, selected?: boolean, icon?: string, meta?: string, metaIcon?: string, children?: import('svelte').Snippet, [attribute: string]: any}} */
  let { tone = "neutral", selected = false, icon, meta, metaIcon, children, ...rest } = $props();
</script>

<span class="chip" class:selected style:--tone={TONES[tone]} {...rest}>
  {#if icon}<Icon name={icon} inherit={selected} />{/if}
  {@render children?.()}
  {#if meta}<span class="meta">{#if metaIcon}<Icon name={metaIcon} />{/if}{meta}</span>{/if}
</span>

<style>
  .chip {
    display: inline-flex;
    align-items: center;
    gap: calc(6px * var(--ui-scale));
    font: var(--machine-sm);
    font-size: calc(11px * var(--ui-scale));
    color: var(--tone);
    background: var(--surface-chrome);
    border: 1px solid var(--border-subtle);
    padding: 2px calc(7px * var(--ui-scale));
  }

  .selected {
    font: var(--machine-bold);
    font-size: calc(11px * var(--ui-scale));
    color: var(--text-on-accent);
    background: var(--tone);
    border: none;
    padding: calc(3px * var(--ui-scale)) calc(9px * var(--ui-scale));
  }

  .meta {
    display: inline-flex;
    align-items: center;
    gap: calc(4px * var(--ui-scale));
    color: var(--text-faint);
  }
</style>
