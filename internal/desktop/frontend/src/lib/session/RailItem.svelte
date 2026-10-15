<script>
  import { uiScale } from "../scale.svelte.js";
  import SessionIdentity from "./SessionIdentity.svelte";
  import SessionSticker from "./SessionSticker.svelte";
  import { rowLabel, summaryFacts } from "./rail.js";
  import { sticker, stickerControlLabel, stickerCss, stickerTitle } from "./stickers.js";

  /** @type {{slug?: string, initials?: string, shortcut?: string, name?: string, repos?: {name: string}[], summary?: import("./rail.js").Summary, unseen?: number, idle?: string, selected?: boolean, dragging?: boolean, over?: boolean, onPointerDown?: (event: PointerEvent) => void, onPointerMove?: (event: PointerEvent) => void, onPointerUp?: (event: PointerEvent) => void, onPointerCancel?: (event: PointerEvent) => void, stickerId?: string, stickers?: import("./stickers.js").Sticker[], stickerBusy?: boolean, feedback?: {sequence: number, text: string, failed: boolean} | null, onSelect?: () => void, onSticker?: (event: MouseEvent) => void, onContextMenu?: (event: MouseEvent) => void}} */
  let {
    slug = "",
    initials,
    shortcut = "",
    name = "",
    repos = [],
    summary = {},
    unseen = 0,
    idle = "",
    selected = false,
    dragging = false,
    over = false,
    onPointerDown,
    onPointerMove,
    onPointerUp,
    onPointerCancel,
    stickerId = "",
    stickers = [],
    stickerBusy = false,
    feedback = null,
    onSelect,
    onSticker,
    onContextMenu,
  } = $props();

  let facts = $derived(summaryFacts(summary, unseen, idle));
  let label = $derived(rowLabel(name, repos, facts));
  let running = $derived(Boolean(summary.running));
  let stickerItem = $derived(sticker(stickerId, stickers));
  let stickerLabel = $derived(stickerControlLabel(name, stickerId, stickers));
  let stickerTooltip = $derived(stickerTitle(stickerId, stickers));

  function fitFeedback(node) {
    const row = node.closest(".row");
    const scrollport = node.closest(".session-list");
    if (!row || !scrollport) return;

    const place = () => {
      node.classList.remove("above");
      const rowBox = row.getBoundingClientRect();
      const scrollBox = scrollport.getBoundingClientRect();
      const gap = (5 * uiScale()) / 100;
      const height = node.getBoundingClientRect().height;
      const below = scrollBox.bottom - rowBox.bottom;
      const above = rowBox.top - scrollBox.top;
      node.classList.toggle("above", below < height + gap && above >= height + gap);
    };

    place();
    scrollport.addEventListener("scroll", place);
    window.addEventListener("resize", place);
    return {
      destroy() {
        scrollport.removeEventListener("scroll", place);
        window.removeEventListener("resize", place);
      },
    };
  }
</script>

<div
  class="row"
  class:selected
  class:running
  class:dragging
  class:over
  data-session-slug={slug}
  role="group"
  aria-label="{name} actions"
  oncontextmenu={onContextMenu}>
  <button
    type="button"
    class="item"
    aria-current={selected ? "page" : undefined}
    aria-label={label}
    title={label}
    onpointerdown={onPointerDown}
    onpointermove={onPointerMove}
    onpointerup={onPointerUp}
    onpointercancel={onPointerCancel}
    onlostpointercapture={onPointerCancel}
    onmousedown={(event) => event.preventDefault()}
    onclick={(event) => {
      if (event.detail === 0) onSelect?.();
    }}>
    <SessionIdentity {initials} {shortcut} {name} {repos} {summary} {unseen} {idle} {selected} />
  </button>

  <button
    type="button"
    class="sticker"
    class:empty={!stickerItem}
    aria-label={stickerLabel}
    aria-busy={stickerBusy ? "true" : undefined}
    title={stickerTooltip}
    style:color={stickerItem ? stickerCss(stickerItem.colour) : undefined}
    onmousedown={(event) => event.preventDefault()}
    onclick={onSticker}>
    <SessionSticker id={stickerId} {stickers} />
  </button>

  {#if feedback}
    {#key feedback.sequence}
      <div
        class="feedback"
        class:failed={feedback.failed}
        role={feedback.failed ? "alert" : "status"}
        aria-live={feedback.failed ? undefined : "polite"}
        aria-atomic={feedback.failed ? undefined : "true"}
        use:fitFeedback>
        {feedback.text}
      </div>
    {/key}
  {/if}
</div>

<style>
  .row {
    position: relative;
    display: flex;
    align-items: stretch;
    width: 100%;
    flex: none;
    border: 1px solid transparent;
    background: transparent;
  }

  .row:hover:not(.selected) {
    border-color: var(--border-default);
    background: var(--surface-raised);
  }

  .selected {
    border-color: var(--accent-action);
    background: var(--surface-raised);
    box-shadow: var(--shadow-focus);
  }

  .dragging {
    opacity: 0.4;
  }

  .over {
    box-shadow: inset 0 0 0 1px var(--accent-action);
  }

  .item {
    touch-action: none;
    user-select: none;
    display: flex;
    align-items: center;
    gap: calc(10px * var(--ui-scale));
    min-width: 0;
    flex: 1;
    padding: calc(8px * var(--ui-scale)) calc(4px * var(--ui-scale)) calc(8px * var(--ui-scale)) calc(8px * var(--ui-scale));
    cursor: pointer;
    border: 0;
    background: transparent;
    font: inherit;
    color: inherit;
    text-align: left;
  }

  .item:focus-visible,
  .sticker:focus-visible {
    outline: 2px solid var(--accent-action);
    outline-offset: -2px;
  }

  .row:hover :global(.name) {
    color: var(--text-primary);
  }

  .sticker {
    width: calc(24px * var(--ui-scale));
    height: calc(24px * var(--ui-scale));
    flex: none;
    align-self: flex-start;
    display: flex;
    align-items: center;
    justify-content: center;
    padding: 0;
    border: 1px solid transparent;
    background: color-mix(in srgb, currentColor 8%, transparent);
    font: 400 calc(15px * var(--ui-scale))/1 var(--font-machine);
    cursor: pointer;
    margin: calc(4px * var(--ui-scale)) 2px 0 0;
  }

  .sticker:hover {
    border-color: currentColor;
  }

  .sticker.empty {
    color: var(--text-faint);
    background: transparent;
  }

  .sticker[aria-busy="true"] {
    opacity: 0.6;
  }

  .feedback {
    position: absolute;
    right: 0;
    top: calc(100% + calc(5px * var(--ui-scale)));
    width: max-content;
    max-width: min(calc(220px * var(--ui-scale)), 100%);
    z-index: 10;
    box-sizing: border-box;
    padding: calc(5px * var(--ui-scale)) calc(7px * var(--ui-scale));
    border: 1px solid var(--border-default);
    background: var(--surface-raised);
    box-shadow: var(--shadow-focus);
    color: var(--text-primary);
    font: var(--machine-xs);
    white-space: normal;
    overflow-wrap: anywhere;
  }

  .feedback.failed {
    border-color: var(--state-failed);
  }

  .feedback:global(.above) {
    top: auto;
    bottom: calc(100% + calc(5px * var(--ui-scale)));
  }
</style>
