<script>
  import { tick } from "svelte";
  import Button from "../core/Button.svelte";
  import CapsLabel from "../core/CapsLabel.svelte";
  import StickerIcon from "../session/StickerIcon.svelte";
  import { STICKER_COLOURS, STICKER_SYMBOLS, stickerCss, stickerName } from "../session/stickers.js";
  import { DUPLICATE, addSticker, duplicates, removeSticker, updateSticker } from "./stickers.js";
  import TabHeader from "./TabHeader.svelte";

  /** @type {{panel: ReturnType<typeof import("./settings.svelte.js").settings>, labelledby?: string}} */
  let { panel, labelledby } = $props();

  let form = $derived(panel.form);
  let selected = $state(0);
  let current = $derived(form.stickers[selected]);
  let repeated = $derived(duplicates(form.stickers));
  /** @type {HTMLInputElement[]} */
  const inputs = [];

  /** @param {number} index */
  const errorFor = (index) => panel.fields[`stickers.${index}`] ?? (repeated.has(index) ? DUPLICATE : "");

  async function add() {
    form.stickers = addSticker(form.stickers);
    selected = form.stickers.length - 1;
    panel.clearStickerErrors();
    await tick();
    inputs[selected]?.focus();
  }

  /** @param {number} index */
  function remove(index) {
    form.stickers = removeSticker(form.stickers, index);
    panel.clearStickerErrors();
    if (selected >= index) selected = Math.max(0, selected - 1);
  }

  /** @param {Partial<import("../session/stickers.js").Sticker>} patch */
  function change(patch) {
    form.stickers = updateSticker(form.stickers, selected, patch);
    panel.clearStickerErrors();
  }
</script>

<TabHeader title="Stickers" id={labelledby}>
  Tag sessions so you can spot them in the rail. Each sticker is a colour and a symbol, plus a short meaning
  shown on hover. Fewer is easier to read at a glance.
</TabHeader>

<div class="split">
  <div class="list">
    <ul class="rows" aria-label="Session stickers">
      {#each form.stickers as item, index (index)}
        {@const error = errorFor(index)}
        <li class="row" class:selected={index === selected}>
          <button
            type="button"
            class="glyph"
            aria-label="Edit {stickerName(item)}"
            aria-pressed={index === selected}
            onclick={() => (selected = index)}>
            <StickerIcon symbol={item.symbol} colour={item.colour} size={16} />
          </button>
          <div class="label">
            <input
              aria-label="{stickerName(item)} meaning"
              placeholder="What it means"
              bind:this={inputs[index]}
              bind:value={item.label}
              onfocus={() => (selected = index)}
              oninput={panel.clearStickerErrors} />
            {#if error}<span class="error">{error}</span>{/if}
          </div>
          <button
            type="button"
            class="remove"
            aria-label="Remove {item.label || stickerName(item)}"
            onclick={() => remove(index)}>Remove</button>
        </li>
      {/each}
    </ul>
    <div>
      <Button variant="cube" icon="new" onclick={add}>Add sticker</Button>
    </div>
    <span class="aside">Removing a sticker takes it off any session that has it.</span>
  </div>

  {#if current}
    <div class="editor" aria-label="Edit sticker" role="group">
      <div class="preview">
        <span class="big"><StickerIcon symbol={current.symbol} colour={current.colour} size={28} /></span>
        <div class="names">
          <span class="meaning">{current.label || "Untitled sticker"}</span>
          <span class="name">{stickerName(current)}</span>
        </div>
      </div>
      <div class="pick">
        <CapsLabel id="sticker-colour">Colour</CapsLabel>
        <div class="colours" role="group" aria-labelledby="sticker-colour">
          {#each STICKER_COLOURS as colour (colour)}
            <button
              type="button"
              class="swatch"
              class:on={current.colour === colour}
              title={colour}
              aria-label={colour}
              aria-pressed={current.colour === colour}
              style:background={stickerCss(colour)}
              onclick={() => change({ colour })}></button>
          {/each}
        </div>
      </div>
      <div class="pick">
        <CapsLabel id="sticker-symbol">Symbol</CapsLabel>
        <div class="symbols" role="group" aria-labelledby="sticker-symbol">
          {#each Object.entries(STICKER_SYMBOLS) as [symbol, drawn] (symbol)}
            <button
              type="button"
              class="symbol"
              class:on={current.symbol === symbol}
              title={drawn.name}
              aria-label={drawn.name}
              aria-pressed={current.symbol === symbol}
              style:--ink={stickerCss(current.colour)}
              onclick={() => change({ symbol })}>
              <StickerIcon {symbol} size={14} />
            </button>
          {/each}
        </div>
      </div>
    </div>
  {/if}
</div>

<style>
  .split {
    display: grid;
    grid-template-columns: minmax(0, 1fr) calc(300px * var(--ui-scale));
    gap: calc(28px * var(--ui-scale));
    align-items: start;
  }

  .list {
    display: flex;
    flex-direction: column;
    gap: calc(10px * var(--ui-scale));
  }

  .rows {
    display: flex;
    flex-direction: column;
    margin: 0;
    padding: 0;
    list-style: none;
    border-top: 1px solid var(--border-subtle);
  }

  .row {
    display: flex;
    align-items: center;
    gap: calc(12px * var(--ui-scale));
    padding: calc(7px * var(--ui-scale)) calc(10px * var(--ui-scale));
    border-bottom: 1px solid var(--border-subtle);
  }

  .row.selected {
    background: var(--wash-selected);
    box-shadow: inset calc(2px * var(--ui-scale)) 0 0 var(--accent-action);
  }

  .glyph {
    width: calc(26px * var(--ui-scale));
    height: calc(26px * var(--ui-scale));
    flex: none;
    display: flex;
    align-items: center;
    justify-content: center;
    border: 0;
    background: none;
    padding: 0;
    cursor: pointer;
  }

  .glyph:focus-visible,
  .swatch:focus-visible,
  .symbol:focus-visible {
    outline: 1px solid var(--accent-action);
    outline-offset: calc(2px * var(--ui-scale));
  }

  .label {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
  }

  input {
    border: 0;
    outline: none;
    background: transparent;
    padding: calc(4px * var(--ui-scale)) 0;
    font: var(--machine-md);
    color: var(--text-primary);
    caret-color: var(--caret);
  }

  input::placeholder {
    color: var(--text-faint);
  }

  .error {
    font: var(--machine-xs);
    color: var(--state-failed);
  }

  .remove {
    border: 0;
    background: none;
    padding: 0;
    font: var(--machine-xs);
    color: var(--text-faint);
    cursor: pointer;
  }

  .remove:hover,
  .remove:focus-visible {
    color: var(--state-failed);
    outline: none;
  }

  .aside {
    font: var(--machine-xs);
    color: var(--text-muted);
  }

  .editor {
    display: flex;
    flex-direction: column;
    gap: calc(16px * var(--ui-scale));
    padding: calc(16px * var(--ui-scale));
    border: 1px solid var(--border-default);
    background: var(--surface-chrome);
  }

  .preview {
    display: flex;
    align-items: center;
    gap: calc(12px * var(--ui-scale));
  }

  .big {
    width: calc(44px * var(--ui-scale));
    height: calc(44px * var(--ui-scale));
    flex: none;
    display: flex;
    align-items: center;
    justify-content: center;
    background: var(--surface-app);
    border: 1px solid var(--border-subtle);
  }

  .names {
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: calc(4px * var(--ui-scale));
  }

  .meaning {
    font: var(--machine-md);
    color: var(--text-primary);
    overflow-wrap: anywhere;
  }

  .name {
    font: var(--machine-xs);
    color: var(--text-muted);
  }

  .pick {
    display: flex;
    flex-direction: column;
    gap: calc(8px * var(--ui-scale));
  }

  .colours,
  .symbols {
    display: grid;
    gap: calc(6px * var(--ui-scale));
  }

  .colours {
    grid-template-columns: repeat(6, minmax(0, 1fr));
  }

  .symbols {
    grid-template-columns: repeat(7, minmax(0, 1fr));
  }

  .swatch,
  .symbol {
    aspect-ratio: 1;
    border-radius: 0;
    padding: 0;
    cursor: pointer;
  }

  .swatch {
    border: 0;
  }

  .swatch.on {
    outline: 1px solid var(--text-primary);
    outline-offset: calc(2px * var(--ui-scale));
  }

  .symbol {
    display: flex;
    align-items: center;
    justify-content: center;
    border: 1px solid var(--border-subtle);
    background: transparent;
    color: var(--ink);
  }

  .symbol:hover {
    border-color: var(--text-muted);
  }

  .symbol.on {
    border-color: var(--ink);
    background: var(--surface-app);
  }
</style>
