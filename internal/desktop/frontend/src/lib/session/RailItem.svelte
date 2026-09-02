<script>
  import { repositoryLine, rowLabel, summaryFacts } from "./activity.js";
  import StickerIcon from "./StickerIcon.svelte";
  import { sticker, stickerControlLabel, stickerTitle } from "./stickers.js";

  /** @type {{initials?: string, shortcut?: string, name?: string, repos?: {name: string}[], summary?: {attention?: string, active?: number, coverage?: string, running?: boolean}, unseen?: number, idle?: string, selected?: boolean, stickerId?: string, stickerLabels?: Record<string, string>, stickerBusy?: boolean, feedback?: {sequence: number, text: string, failed: boolean} | null, onSelect?: () => void, onSticker?: (event: MouseEvent) => void, onContextMenu?: (event: MouseEvent) => void}} */
  let {
    initials,
    shortcut = "",
    name = "",
    repos = [],
    summary = {},
    unseen = 0,
    idle = "",
    selected = false,
    stickerId = "",
    stickerLabels = {},
    stickerBusy = false,
    feedback = null,
    onSelect,
    onSticker,
    onContextMenu,
  } = $props();

  let repository = $derived(repositoryLine(repos));
  let facts = $derived(summaryFacts(summary, unseen, idle));
  let label = $derived(rowLabel(name, repos, facts));
  let running = $derived(Boolean(summary.running));
  let stickerItem = $derived(sticker(stickerId));
  let stickerLabel = $derived(stickerControlLabel(name, stickerId, stickerLabels));
  let stickerTooltip = $derived(stickerTitle(stickerId, stickerLabels));

  function fitFeedback(node) {
    const row = node.closest(".row");
    const scrollport = node.closest(".session-list");
    if (!row || !scrollport) return;

    const place = () => {
      node.classList.remove("above");
      const rowBox = row.getBoundingClientRect();
      const scrollBox = scrollport.getBoundingClientRect();
      const gap = 5;
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
  role="group"
  aria-label="{name} actions"
  oncontextmenu={onContextMenu}>
  <button
    type="button"
    class="item"
    aria-current={selected ? "page" : undefined}
    aria-label={label}
    title={label}
    onmousedown={(event) => event.preventDefault()}
    onclick={onSelect}>
    <div class="avatar" class:keyed={shortcut}>
      {shortcut || initials}
    </div>

    <div class="text">
      <div class="name">{name}</div>
      <div class="repositories" title={[repository.name, repository.extra].filter(Boolean).join(" ")}>
        {repository.name}{#if repository.extra}<span class="extra">{repository.extra}</span>{/if}
      </div>
      <div class="facts">
        {#each facts as fact (fact.kind)}
          <span class="fact {fact.kind}">
            {#if fact.kind !== "agents" || fact.active}
              <span class="glyph" class:running={fact.active} aria-hidden="true"
                >{fact.kind === "attention" ? "!" : fact.kind === "unseen" ? "◆" : "●"}</span>
            {/if}
            {fact.label}
          </span>
        {/each}
      </div>
    </div>
  </button>

  <button
    type="button"
    class="sticker"
    class:empty={!stickerItem}
    aria-label={stickerLabel}
    aria-busy={stickerBusy ? "true" : undefined}
    title={stickerTooltip}
    style:color={stickerItem?.css}
    onmousedown={(event) => event.preventDefault()}
    onclick={onSticker}>
    {#if stickerItem}
      <StickerIcon id={stickerItem.id} />
    {:else}
      <StickerIcon id="star" outline />
    {/if}
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

  .item {
    display: flex;
    align-items: center;
    gap: 10px;
    min-width: 0;
    flex: 1;
    padding: 8px 4px 8px 8px;
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

  .row:hover .name {
    color: var(--text-primary);
  }

  .avatar {
    position: relative;
    width: 30px;
    height: 30px;
    flex: none;
    display: flex;
    align-items: center;
    justify-content: center;
    font: 700 13px var(--font-machine);
    background: transparent;
    border: 1px solid var(--border-subtle);
    color: var(--text-faint);
  }

  .running .avatar {
    background: var(--surface-raised);
    border-color: var(--border-default);
    color: var(--text-secondary);
  }

  .selected .avatar {
    background: var(--accent-action);
    border-color: var(--accent-action);
    color: var(--text-on-accent);
  }

  .keyed {
    font: var(--machine-md);
    font-size: 11px;
  }

  .text {
    min-width: 0;
    flex: 1;
  }

  .name,
  .repositories {
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .name {
    font: var(--machine-md);
    font-size: 11px;
    color: var(--text-muted);
  }

  .running .name {
    color: var(--text-secondary);
  }

  .selected .name {
    font: var(--machine-bold);
    font-size: 11px;
    color: var(--text-primary);
  }

  .repositories,
  .facts {
    font: var(--machine-xs);
    font-size: 9.5px;
    margin-top: 2px;
    color: var(--text-faint);
  }

  .selected .repositories,
  .selected .facts {
    color: var(--text-muted);
  }

  .extra {
    margin-left: 0.5ch;
    color: var(--text-muted);
  }

  .facts {
    display: flex;
    flex-wrap: wrap;
    column-gap: 6px;
    row-gap: 1px;
  }

  .fact {
    flex: none;
  }

  .fact.attention {
    color: var(--state-waiting);
  }

  .glyph.running {
    color: var(--state-running);
  }

  .fact.unseen {
    color: var(--state-guided);
  }

  .sticker {
    width: 24px;
    height: 24px;
    flex: none;
    align-self: flex-start;
    display: flex;
    align-items: center;
    justify-content: center;
    padding: 0;
    border: 1px solid transparent;
    background: color-mix(in srgb, currentColor 8%, transparent);
    font: 400 15px/1 var(--font-machine);
    cursor: pointer;
    margin: 4px 2px 0 0;
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
    top: calc(100% + 5px);
    width: max-content;
    max-width: min(220px, 100%);
    z-index: 10;
    box-sizing: border-box;
    padding: 5px 7px;
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

  .feedback.above {
    top: auto;
    bottom: calc(100% + 5px);
  }
</style>
