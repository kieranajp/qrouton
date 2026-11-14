<script>
  import CapsLabel from "../core/CapsLabel.svelte";
  import Icon from "../core/Icon.svelte";

  const HELP_TONES = {
    muted: "var(--text-muted)",
    success: "var(--state-success)",
    waiting: "var(--state-waiting)",
    failed: "var(--state-failed)",
  };

  /** @type {{label?: string, value?: string, placeholder?: string, help?: string, helpLiteral?: string, helpTone?: 'muted'|'success'|'waiting'|'failed', icon?: import("../core/icons.js").IconName, multiline?: boolean, valueVoice?: 'prose'|'literal', clearLabel?: string, trailing?: import('svelte').Snippet, [attribute: string]: any}} */
  let {
    label,
    value = $bindable(""),
    placeholder,
    help,
    helpLiteral,
    helpTone = "muted",
    icon,
    multiline = false,
    valueVoice = "prose",
    clearLabel,
    trailing,
    ...rest
  } = $props();

  let input = $state(/** @type {HTMLInputElement | HTMLTextAreaElement | undefined} */ (undefined));

  function clear() {
    value = "";
    input?.focus();
  }
</script>

<div class="field">
  {#if label}<CapsLabel>{label}</CapsLabel>{/if}
  <div class="row">
    <div class="input" class:literal={valueVoice === "literal"}>
      {#if icon}<span class="icon"><Icon name={icon} /></span>{/if}
      {#if multiline}
        <textarea bind:this={input} bind:value {placeholder} {...rest}></textarea>
      {:else}
        <input type="text" bind:this={input} bind:value {placeholder} {...rest} />
      {/if}
      {#if clearLabel && value}
        <button type="button" class="clear" aria-label={clearLabel} title={clearLabel} onclick={clear}>
          <Icon name="close" />
        </button>
      {/if}
    </div>
    {@render trailing?.()}
  </div>
  {#if help}<span class="help" style:color={HELP_TONES[helpTone]}>{#if helpTone === "success"}<Icon name="succeeded" />{/if}{help}{#if helpLiteral}&nbsp;<code
        >{helpLiteral}</code
      >{/if}</span>{/if}
</div>

<style>
  .field {
    display: flex;
    flex-direction: column;
    gap: calc(7px * var(--ui-scale));
  }

  .row {
    display: flex;
    gap: calc(8px * var(--ui-scale));
  }

  .input {
    flex: 1;
    display: flex;
    align-items: center;
    gap: calc(9px * var(--ui-scale));
    border: 1px solid var(--border-default);
    background: var(--surface-chrome);
    padding: calc(9px * var(--ui-scale)) calc(12px * var(--ui-scale));
  }

  .input:focus-within {
    border-color: var(--accent-action);
    box-shadow: var(--shadow-focus);
  }

  .icon {
    flex: none;
    color: var(--text-faint);
  }

  .clear {
    display: flex;
    flex: none;
    border: 0;
    padding: calc(4px * var(--ui-scale));
    margin: calc(-4px * var(--ui-scale));
    background: transparent;
    color: var(--text-muted);
    cursor: pointer;
  }

  .clear:hover {
    color: var(--text-primary);
  }

  input,
  textarea {
    flex: 1;
    min-width: 0;
    border: 0;
    outline: none;
    background: transparent;
    padding: 0;
    font: var(--machine-md);
    color: var(--text-primary);
    caret-color: var(--caret);
  }

  .literal input,
  .literal textarea {
    font: var(--literal);
    color: var(--text-secondary);
  }

  textarea {
    min-height: calc(52px * var(--ui-scale));
    resize: none;
  }

  input::placeholder,
  textarea::placeholder {
    color: var(--text-faint);
  }

  .help {
    font: var(--machine-sm);
    font-size: calc(11px * var(--ui-scale));
  }

  .help :global(.icon) {
    display: inline-block;
    vertical-align: middle;
    margin: calc(-2px * var(--ui-scale)) calc(5px * var(--ui-scale)) 0 0;
  }

  code {
    font: var(--literal);
    color: var(--text-secondary);
  }
</style>
