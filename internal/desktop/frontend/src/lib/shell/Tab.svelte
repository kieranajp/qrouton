<script>
  import StatusDot from "../core/StatusDot.svelte";
  import ArtifactTag from "../core/ArtifactTag.svelte";
  import { tabLabel } from "./tabs.js";

  // An unfocused tab that cannot report a red test run is one you must click to
  // trust, so the process's state rides along with the label.
  /** @type {{label?: string, badge?: string, artifact?: string, status?: 'succeeded'|'running'|'failed'|'waiting'|'idle', selected?: boolean, closable?: boolean, dragging?: boolean, over?: boolean, onSelect?: () => void, onClose?: () => void, onDragStart?: () => void, onDragOver?: () => void, onDragLeave?: () => void, onDrop?: () => void, onDragEnd?: () => void, [attribute: string]: any}} */
  let {
    label,
    badge,
    artifact,
    status,
    selected = false,
    closable = true,
    dragging = false,
    over = false,
    onSelect,
    onClose,
    onDragStart,
    onDragOver,
    onDragLeave,
    onDrop,
    onDragEnd,
    ...rest
  } = $props();

  let whole = $derived(tabLabel({ badge, label }));

  /** @param {DragEvent} event */
  function lift(event) {
    // A drag carrying no data never starts. Which tab is moving is the strip's
    // own state, so the payload is only ever read by whatever it is dropped on.
    if (event.dataTransfer) {
      event.dataTransfer.setData("text/plain", whole);
      event.dataTransfer.effectAllowed = "move";
    }
    onDragStart?.();
  }

  /** @param {DragEvent} event */
  function hover(event) {
    event.preventDefault();
    if (event.dataTransfer) event.dataTransfer.dropEffect = "move";
    onDragOver?.();
  }

  /** @param {DragEvent} event */
  function release(event) {
    event.preventDefault();
    onDrop?.();
  }

  /** @param {MouseEvent} event */
  function auxiliary(event) {
    if (event.button !== 1 || !closable) return;
    event.preventDefault();
    onClose?.();
  }
</script>

<div
  class="tab"
  class:selected
  class:dragging
  class:over
  title={whole}
  draggable="true"
  ondragstart={lift}
  ondragover={hover}
  ondragleave={() => onDragLeave?.()}
  ondrop={release}
  ondragend={() => onDragEnd?.()}
  onauxclick={auxiliary}
  {...rest}>
  <button type="button" class="select" onclick={onSelect}>
    {#if status}<StatusDot state={status} size={7} />{/if}
    {#if badge}<ArtifactTag kind={artifact} id={badge} />{/if}
    <span class="label">{label}</span>
  </button>
  {#if closable}
    <button type="button" class="close" aria-label="Close tab" onclick={() => onClose?.()}
      >&#10005;</button>
  {/if}
</div>

<style>
  .tab {
    display: flex;
    align-items: stretch;
    gap: 9px;
    padding: 0 14px;
    border-right: 1px solid var(--border-subtle);
    border-bottom: 2px solid transparent;
    background: transparent;
    flex: 1 1 auto;
    min-width: 0;
    max-width: 210px;
    overflow: hidden;
  }

  .dragging {
    opacity: 0.4;
  }

  .over {
    box-shadow: inset 0 0 0 1px var(--accent-action);
  }

  /* Selection is blue and separate from status. */
  .selected {
    border-bottom-color: var(--accent-action);
    background: var(--wash-selected);
  }

  .select,
  .close {
    display: flex;
    align-items: center;
    background: none;
    border: 0;
    padding: 0;
    font: inherit;
    color: inherit;
    text-align: inherit;
    cursor: pointer;
  }

  .select {
    gap: 8px;
    min-width: 0;
    flex: 1 1 auto;
  }

  .label {
    font: var(--machine-sm);
    font-size: 11.5px;
    color: var(--text-muted);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .selected .label {
    color: var(--text-primary);
  }

  .close {
    font-size: 11px;
    color: var(--text-faint);
  }
</style>
