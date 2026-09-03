<script>
  /** Below 24px the hatch averages to mid-grey, so the flat cut is automatic.
   * @type {{size?: number, face?: string, back?: string, [attribute: string]: any}} */
  let { size = 40, face = "var(--accent-label)", back = "var(--ctp-surface-2)", ...rest } = $props();

  let off = $derived(Math.round(size * 0.18));
  let inner = $derived(size - off);
  let hatch = $derived(
    size >= 24
      ? `repeating-linear-gradient(45deg, rgba(24,25,38,.45) 0 ${Math.max(1, inner * 0.025)}px,` +
        ` transparent ${Math.max(1, inner * 0.025)}px ${Math.max(3, inner * 0.07)}px)`
      : "none",
  );
</script>

<span class="mark" style:width="{size}px" style:height="{size}px" {...rest}>
  <span
    class="square back"
    style:left="{off}px"
    style:top="{off}px"
    style:width="{inner}px"
    style:height="{inner}px"
    style:background-color={back}
    style:background-image={hatch}
  ></span>
  <span
    class="square face"
    style:width="{inner}px"
    style:height="{inner}px"
    style:background={face}
  ></span>
</span>

<style>
  .mark {
    position: relative;
    display: inline-block;
    flex: none;
  }

  .square {
    position: absolute;
  }

  .face {
    left: 0;
    top: 0;
  }
</style>
