<script>
  import { ICONS } from "./icons.js";

  const COLOURS = {
    new: "var(--accent-label)",
    agent: "var(--accent-action)",
    shell: "var(--accent-action)",
    pending: "var(--text-faint)",
    excluded: "var(--text-muted)",
    succeeded: "var(--state-success)",
    failed: "var(--state-failed)",
    running: "var(--state-running)",
    waiting: "var(--state-waiting)",
    editing: "var(--role-editing)",
    reference: "var(--role-reference)",
  };

  /** @type {{name: string, inherit?: boolean}} */
  let { name, inherit = false } = $props();

  let paths = $derived(ICONS[name]);
</script>

<svg
  class="icon"
  viewBox="0 0 16 16"
  data-icon={name}
  aria-hidden="true"
  style:color={inherit ? undefined : COLOURS[name]}>
  {#if paths?.stroke}
    <path
      d={paths.stroke}
      fill="none"
      stroke="currentColor"
      stroke-width="1.5"
      stroke-linecap="square"
      stroke-linejoin="miter" />
  {/if}
  {#if paths?.fill}
    <path d={paths.fill} fill="currentColor" />
  {/if}
</svg>

<style>
  .icon {
    display: block;
    flex: none;
    width: calc(16px * var(--ui-scale));
    height: calc(16px * var(--ui-scale));
  }
</style>
