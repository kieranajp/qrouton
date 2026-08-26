<script>
  import { onDestroy } from "svelte";
  import { latestPerFrame } from "../frame.js";

  /** @type {{size: number, min: number, max: number, onResize: (size: number) => void, onCommit: (size: number) => void, onReset?: () => void, label?: string}} */
  let { size, min, max, onResize, onCommit, onReset, label = "Resize pane" } = $props();

  let origin = 0;
  let grabbed = 0;
  let latest = 0;
  let dragging = $state(false);
  const scheduled = latestPerFrame((value) => onResize(value));

  const clamp = (value) => Math.min(max, Math.max(min, value));
  const sizeAt = (event) => clamp(grabbed - (event.clientX - origin));

  function grab(event) {
    if (event.button !== 0) return;
    origin = event.clientX;
    grabbed = size;
    latest = size;
    dragging = true;
    event.currentTarget.setPointerCapture(event.pointerId);
  }

  // The pane being sized sits to the right of the border, so it grows as the
  // pointer moves left.
  function drag(event) {
    if (!dragging) return;
    latest = sizeAt(event);
    scheduled.schedule(latest);
  }

  function release(event) {
    if (!dragging) return;
    if (event.type === "pointerup") latest = sizeAt(event);
    dragging = false;
    scheduled.cancel();
    onCommit(latest);
    if (event.currentTarget.hasPointerCapture(event.pointerId)) {
      event.currentTarget.releasePointerCapture(event.pointerId);
    }
  }

  function nudge(event) {
    const step = event.shiftKey ? 40 : 8;
    if (event.key === "ArrowLeft") onCommit(clamp(size + step));
    else if (event.key === "ArrowRight") onCommit(clamp(size - step));
    else return;
    event.preventDefault();
  }

  onDestroy(scheduled.cancel);

  // Pointer capture keeps the events coming, but the panes either side would
  // still select their text under the drag.
  $effect(() => {
    if (!dragging) return;
    const style = document.body.style;
    style.userSelect = "none";
    style.cursor = "col-resize";
    return () => {
      style.userSelect = "";
      style.cursor = "";
    };
  });
</script>

<!-- A separator counts as non-interactive to the linter, but a focusable one is
     a widget in ARIA and this is the window-splitter case. -->
<!-- svelte-ignore a11y_no_noninteractive_tabindex -->
<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
<div
  class="splitter"
  class:dragging
  role="separator"
  tabindex="0"
  aria-orientation="vertical"
  aria-label={label}
  aria-valuenow={size}
  aria-valuemin={min}
  aria-valuemax={max}
  onpointerdown={grab}
  onpointermove={drag}
  onpointerup={release}
  onpointercancel={release}
  onkeydown={nudge}
  ondblclick={onReset}>
</div>

<style>
  /* One pixel is the border the panes had before; the grab area is the wider
     thing the pseudo-element draws nothing into. */
  .splitter {
    position: relative;
    width: 1px;
    flex: none;
    background: var(--border-subtle);
    cursor: col-resize;
  }

  .splitter::after {
    content: "";
    position: absolute;
    inset: 0 -4px;
  }

  .splitter:hover,
  .splitter:focus-visible,
  .splitter.dragging {
    background: var(--border-accent);
    outline: none;
  }
</style>
