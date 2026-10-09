<script>
  import { onMount } from "svelte";

  /** @type {{query?: string, count?: number, current?: number, field?: HTMLInputElement, onQuery?: (value: string) => void, onPrevious?: () => void, onNext?: () => void, onClose?: () => void}} */
  let {
    query = "",
    count = 0,
    current = -1,
    field = $bindable(),
    onQuery,
    onPrevious,
    onNext,
    onClose,
  } = $props();

  let tally = $derived(query ? (count ? `${current + 1} / ${count}` : "No results") : "");

  onMount(() => {
    field?.focus();
    field?.select();
  });

  function key(event) {
    if (event.key === "Enter") {
      event.preventDefault();
      (event.shiftKey ? onPrevious : onNext)?.();
    } else if (event.key === "Escape") {
      event.preventDefault();
      event.stopPropagation();
      onClose?.();
    }
  }

  function move(next) {
    next?.();
    field?.focus();
  }
</script>

<div class="find" role="search">
  <span class="glyph" aria-hidden="true">⌕</span>
  <input
    bind:this={field}
    type="text"
    value={query}
    aria-label="Find in document"
    placeholder="Find in document"
    oninput={(event) => onQuery?.(event.currentTarget.value)}
    onkeydown={key} />
  <span class="tally" role="status" aria-live="polite">{tally}</span>
  <button type="button" aria-label="Previous match" title="Previous match" onclick={() => move(onPrevious)}
    >↑</button>
  <button type="button" aria-label="Next match" title="Next match" onclick={() => move(onNext)}
    >↓</button>
  <button type="button" aria-label="Close find" title="Close find" onclick={onClose}>×</button>
</div>

<style>
  .find {
    flex: none;
    display: flex;
    align-items: center;
    gap: calc(7px * var(--ui-scale));
    padding: calc(7px * var(--ui-scale)) calc(9px * var(--ui-scale));
    background: var(--surface-chrome);
    border-bottom: 1px solid var(--border-subtle);
  }

  .glyph {
    color: var(--text-faint);
    font: var(--machine-md);
  }

  input {
    flex: 1;
    min-width: calc(72px * var(--ui-scale));
    border: 1px solid var(--border-default);
    outline: none;
    background: var(--surface-terminal);
    padding: calc(6px * var(--ui-scale)) calc(8px * var(--ui-scale));
    font: var(--machine-md);
    color: var(--text-primary);
    caret-color: var(--caret);
  }

  input:focus {
    border-color: var(--accent-action);
    box-shadow: var(--shadow-focus);
  }

  input::placeholder {
    color: var(--text-faint);
  }

  .tally {
    width: calc(66px * var(--ui-scale));
    flex: none;
    font: var(--machine-sm);
    font-size: calc(10.5px * var(--ui-scale));
    color: var(--text-muted);
    text-align: right;
    white-space: nowrap;
  }

  button {
    width: calc(25px * var(--ui-scale));
    height: calc(25px * var(--ui-scale));
    flex: none;
    border: 1px solid transparent;
    background: transparent;
    padding: 0;
    font: var(--machine-md);
    color: var(--text-muted);
    cursor: pointer;
  }

  button:hover {
    border-color: var(--border-default);
    color: var(--text-primary);
  }
</style>
