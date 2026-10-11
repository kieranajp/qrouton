<script>
  import StickerIcon from "../session/StickerIcon.svelte";
  import { stickerName } from "../session/stickers.js";
  import TabHeader from "./TabHeader.svelte";

  /** @type {{panel: ReturnType<typeof import("./settings.svelte.js").settings>, labelledby?: string}} */
  let { panel, labelledby } = $props();

  let form = $derived(panel.form);
</script>

<TabHeader title="Stickers" id={labelledby}>
  Tag sessions so you can spot them in the rail. Each sticker is a colour and a symbol, plus a short meaning
  shown on hover.
</TabHeader>

<ul class="rows" aria-label="Session stickers">
  {#each form.stickers as item, index (index)}
    {@const error = panel.fields[`stickers.${index}`]}
    <li class="row">
      <span class="glyph"><StickerIcon symbol={item.symbol} colour={item.colour} size={16} /></span>
      <div class="label">
        <input aria-label="{stickerName(item)} meaning" placeholder="What it means" bind:value={item.label} />
        {#if error}<span class="error">{error}</span>{/if}
      </div>
    </li>
  {/each}
</ul>

<style>
  .rows {
    display: flex;
    flex-direction: column;
    max-width: calc(640px * var(--ui-scale));
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

  .glyph {
    width: calc(26px * var(--ui-scale));
    height: calc(26px * var(--ui-scale));
    flex: none;
    display: flex;
    align-items: center;
    justify-content: center;
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
</style>
