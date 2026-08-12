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
    gap: 7px;
  }

  .row {
    display: flex;
    gap: 8px;
  }

  .input {
    flex: 1;
    display: flex;
    align-items: center;
    gap: 9px;
    border: 1px solid var(--border-default);
    background: var(--surface-chrome);
    padding: 9px 12px;
  }

  .input:focus-within {
    border-color: var(--accent-action);
    box-shadow: var(--shadow-focus);
  }

  .icon {
    flex: none;
    font-size: 12px;
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
    min-height: 52px;
    resize: none;
  }

  input::placeholder,
  textarea::placeholder {
    color: var(--text-faint);
  }

  .help {
    font: var(--machine-sm);
    font-size: 11px;
  }

  code {
    font: var(--literal);
    color: var(--text-secondary);
  }
</style>
