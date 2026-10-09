<script>
  import CapsLabel from "../core/CapsLabel.svelte";

  const HELP_TONES = {
    muted: "var(--text-muted)",
    success: "var(--state-success)",
    waiting: "var(--state-waiting)",
    failed: "var(--state-failed)",
  };

  /** @type {{label?: string, value?: string, placeholder?: string, help?: string, helpLiteral?: string, helpTone?: 'muted'|'success'|'waiting'|'failed', icon?: string, multiline?: boolean, valueVoice?: 'prose'|'literal', trailing?: import('svelte').Snippet, [attribute: string]: any}} */
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
    trailing,
    ...rest
  } = $props();
</script>

<div class="field">
  {#if label}<CapsLabel>{label}</CapsLabel>{/if}
  <div class="row">
    <div class="input" class:literal={valueVoice === "literal"}>
      {#if icon}<span class="icon" aria-hidden="true">{icon}</span>{/if}
      {#if multiline}
        <textarea bind:value {placeholder} {...rest}></textarea>
      {:else}
        <input type="text" bind:value {placeholder} {...rest} />
      {/if}
    </div>
    {@render trailing?.()}
  </div>
  {#if help}<span class="help" style:color={HELP_TONES[helpTone]}>{help}{#if helpLiteral}&nbsp;<code
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
    font-size: calc(12px * var(--ui-scale));
    color: var(--text-faint);
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

  code {
    font: var(--literal);
    color: var(--text-secondary);
  }
</style>
