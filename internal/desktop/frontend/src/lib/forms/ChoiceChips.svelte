<script>
  import CapsLabel from "../core/CapsLabel.svelte";
  import Chip from "../core/Chip.svelte";

  /** @type {{label?: string, options?: string[], value?: string|string[], multiple?: boolean, help?: string, helpLiteral?: string, onSelect?: (option: string) => void, [attribute: string]: any}} */
  let { label, options = [], value, multiple = false, help, helpLiteral, onSelect, ...rest } =
    $props();

  let chosen = $derived(Array.isArray(value) ? value : [value]);
</script>

<div class="choices" {...rest}>
  {#if label}<CapsLabel>{label}</CapsLabel>{/if}
  <div class="options">
    {#each options as option (option)}
      {#if chosen.includes(option)}
        <Chip
          selected
          style="background: var(--accent-action); cursor: pointer; padding: 5px 11px"
          onclick={() => onSelect?.(option)}>{option}</Chip>
      {:else}
        <span class="option" onclick={() => onSelect?.(option)} role="presentation">{option}</span>
      {/if}
    {/each}
  </div>
  {#if help}<span class="help"
      >{help}{#if helpLiteral}&nbsp;<code>{helpLiteral}</code>{/if}</span
    >{/if}
</div>

<style>
  .choices {
    display: flex;
    flex-direction: column;
    gap: 7px;
  }

  .options {
    display: flex;
    flex-wrap: wrap;
    gap: 6px;
  }

  .option {
    font: var(--machine-sm);
    font-size: 11.5px;
    color: var(--text-secondary);
    border: 1px solid var(--border-default);
    padding: 4px 10px;
    cursor: pointer;
  }

  .help {
    font: var(--machine-sm);
    font-size: 11px;
    color: var(--text-muted);
  }

  code {
    font: var(--literal);
    color: var(--text-secondary);
  }
</style>
