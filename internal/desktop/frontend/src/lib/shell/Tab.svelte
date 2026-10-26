<script>
  import Icon from "../core/Icon.svelte";
  import { TAB_STATUSES } from "../bridge/generated.js";
  import ArtifactTag from "../core/ArtifactTag.svelte";
  import { tabLabel } from "./tabs.js";

  /** Status stays visible on unfocused tabs so failed runs remain apparent.
   * @type {{label?: string, badge?: string, artifact?: string, icon?: import("../bridge/generated.js").WindowIcon, status?: import("../bridge/generated.js").TabStatus, selected?: boolean, closable?: boolean, pinned?: boolean, dragging?: boolean, over?: boolean, onSelect?: () => void, onClose?: () => void, onDragStart?: () => void, onDragOver?: () => void, onDragLeave?: () => void, onDrop?: () => void, onDragEnd?: () => void, [attribute: string]: any}} */
  let {
    label,
    badge,
    artifact,
    icon,
    status,
    selected = false,
    closable = true,
    pinned = false,
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
  class:home={pinned}
  class:selected
  class:dragging
  class:over
  title={whole}
  draggable={pinned ? "false" : "true"}
  ondragstart={lift}
  ondragover={hover}
  ondragleave={() => onDragLeave?.()}
  ondrop={release}
  ondragend={() => onDragEnd?.()}
  onauxclick={auxiliary}
  {...rest}>
  <button type="button" class="select" onclick={onSelect}>
    {#if status && status !== TAB_STATUSES.IDLE}<Icon name={status} />{/if}
    {#if badge}<ArtifactTag kind={artifact} id={badge} />{/if}
    {#if icon}<Icon name={icon} />{/if}
    <span class="label">{label}</span>
  </button>
  {#if closable}
    <button type="button" class="close" aria-label="Close tab" onclick={() => onClose?.()}>
      <Icon name="close" />
    </button>
  {/if}
</div>

<style>
  .tab {
    display: flex;
    align-items: stretch;
    gap: calc(9px * var(--ui-scale));
    padding: 0 calc(14px * var(--ui-scale));
    border-right: 1px solid var(--border-subtle);
    border-bottom: 2px solid transparent;
    background: transparent;
    flex: 1 1 auto;
    min-width: 0;
    box-sizing: border-box;
    max-width: calc(160px * var(--ui-scale));
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
    gap: calc(8px * var(--ui-scale));
    min-width: 0;
    flex: 1 1 auto;
  }

  .label {
    font: var(--machine-sm);
    font-size: calc(11.5px * var(--ui-scale));
    color: var(--text-muted);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .selected .label {
    color: var(--text-primary);
  }

  .close {
    color: var(--text-faint);
  }

  /* The session's home: sized to its label, set off from the tabs that come
     and go by a heavier divider, and named in the caps-label voice. */
  .home {
    flex: none;
    max-width: none;
    padding: 0 calc(16px * var(--ui-scale));
    border-right: calc(3px * var(--ui-scale)) solid var(--border-default);
    background: var(--surface-app);
  }

  .home.selected {
    background: var(--wash-selected);
  }

  .home .label {
    font: var(--instruction);
    letter-spacing: var(--instruction-tracking);
    text-transform: uppercase;
  }
</style>
