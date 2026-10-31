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

  const FILLS = { tree: "var(--ctp-teal)" };

  const CHASE_SECONDS = 1.2;

  /** @type {{name: import("./icons.js").IconName, inherit?: boolean}} */
  let { name, inherit = false } = $props();

  let paths = $derived(ICONS[name]);
  // The running icon's dots are drawn clockwise, so staggering them chases.
  let strokes = $derived(name === "running" ? paths.stroke.split(/(?=M)/) : [paths?.stroke].filter(Boolean));
  let chasing = $derived(strokes.length > 1);
</script>

<svg
  class="icon"
  viewBox="0 0 16 16"
  data-icon={name}
  aria-hidden="true"
  style:color={inherit ? undefined : COLOURS[name]}>
  {#each strokes as d, i}
    <path
      {d}
      fill="none"
      stroke="currentColor"
      stroke-width="1.5"
      stroke-linecap="square"
      stroke-linejoin="miter"
      class:dot={chasing}
      style:animation-duration={chasing ? `${CHASE_SECONDS}s` : undefined}
      style:animation-delay={chasing ? `${((i - strokes.length) * CHASE_SECONDS) / strokes.length}s` : undefined} />
  {/each}
  {#if paths?.fill}
    <path d={paths.fill} fill={(!inherit && FILLS[name]) || "currentColor"} />
  {/if}
</svg>

<style>
  .icon {
    display: block;
    flex: none;
    width: calc(16px * var(--ui-scale));
    height: calc(16px * var(--ui-scale));
  }

  .dot {
    animation: chase linear infinite;
  }

  @keyframes chase {
    0% {
      opacity: 1;
    }
    60%,
    100% {
      opacity: 0.25;
    }
  }

  @media (prefers-reduced-motion: reduce) {
    .dot {
      animation: none;
    }
  }
</style>
