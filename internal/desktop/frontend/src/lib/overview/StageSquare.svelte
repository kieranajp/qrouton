<script>
  /** @type {{tone?: string, done?: boolean, live?: boolean}} */
  let { tone = "var(--text-muted)", done = false, live = false } = $props();
</script>

<!-- Filled or hollow, never part-filled; a live lead rings its square. -->
<span class="square" class:done class:live style:--tone={tone} aria-hidden="true"></span>

<style>
  .square {
    flex: none;
    width: calc(9px * var(--ui-scale));
    height: calc(9px * var(--ui-scale));
    box-shadow: inset 0 0 0 1px var(--ctp-surface-2);
  }

  .done {
    background: var(--tone);
    box-shadow: none;
  }

  .live {
    outline: 1px solid var(--tone);
    outline-offset: 2px;
    animation: throb 1.6s ease-in-out infinite;
  }

  @keyframes throb {
    50% {
      outline-color: transparent;
    }
  }

  @media (prefers-reduced-motion: reduce) {
    .live {
      animation: none;
    }
  }
</style>
