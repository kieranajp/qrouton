<script>
  import CapsLabel from "../core/CapsLabel.svelte";

  const HELP_TONES = {
    muted: "var(--text-muted)",
    success: "var(--state-success)",
    waiting: "var(--state-waiting)",
    failed: "var(--state-failed)",
  };

  /** @type {{label?: string, value?: string, placeholder?: string, help?: string, helpTone?: 'muted'|'success'|'waiting'|'failed', focused?: boolean, mono?: boolean, trailing?: import('svelte').Snippet, [attribute: string]: any}} */
  let {
    label,
    value,
    placeholder,
    help,
    helpTone = "muted",
    focused = false,
    mono = false,
    trailing,
    ...rest
  } = $props();
</script>

<div class="field" {...rest}>
  {#if label}<CapsLabel>{label}</CapsLabel>{/if}
  <div class="row">
    <div class="input" class:focused class:mono class:empty={!value}>
      {value || placeholder}
      {#if focused}<span class="caret">&#9612;</span>{/if}
    </div>
    {@render trailing?.()}
  </div>
  {#if help}<span class="help" style:color={HELP_TONES[helpTone]}>{help}</span>{/if}
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
    border: 1px solid var(--border-default);
    background: var(--surface-chrome);
    padding: 9px 12px;
    font: var(--machine-md);
    color: var(--text-primary);
  }

  .mono {
    font: var(--terminal-sm);
  }

  .empty {
    color: var(--text-faint);
  }

  /* Focus draws as selection does; in a keyboard-heavy app they are one idea. */
  .focused {
    border-color: var(--accent-action);
    box-shadow: var(--shadow-focus);
  }

  .caret {
    color: var(--caret);
  }

  .help {
    font: var(--machine-sm);
    font-size: 11px;
  }
</style>
