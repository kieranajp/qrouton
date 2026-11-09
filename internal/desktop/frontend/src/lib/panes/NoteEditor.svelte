<script>
  import CapsLabel from "../core/CapsLabel.svelte";

  /** @type {{text: string, label: string, disabled?: boolean, onNote?: (text: string) => void, onCommit?: () => void}} */
  let { text, label, disabled = false, onNote, onCommit } = $props();
</script>

<label class="note">
  <CapsLabel tone="dim">{label}</CapsLabel>
  <textarea
    rows="2"
    value={text}
    {disabled}
    oninput={(event) => onNote?.(event.currentTarget.value)}
    onblur={() => onCommit?.()}
    onkeydown={(event) => {
      if (event.key === "Enter" && (event.metaKey || event.ctrlKey)) {
        event.preventDefault();
        onCommit?.();
      }
    }}></textarea>
</label>

<style>
  .note {
    display: flex;
    flex-direction: column;
    gap: calc(6px * var(--ui-scale));
  }

  textarea {
    width: 100%;
    box-sizing: border-box;
    min-height: calc(52px * var(--ui-scale));
    padding: calc(8px * var(--ui-scale)) calc(10px * var(--ui-scale));
    border: var(--border-width) solid var(--border-default);
    background: var(--surface-terminal);
    color: var(--text-primary);
    font: var(--machine-md);
    resize: vertical;
  }

  textarea:focus {
    outline: none;
    border-color: var(--accent-action);
  }

</style>
