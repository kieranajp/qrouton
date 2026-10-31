<script>
  import { flash } from "../flash.svelte.js";
  import { copyText } from "../wails.js";

  /** @type {{command: string}} */
  let { command } = $props();

  /** @type {ReturnType<typeof flash<boolean>>} */
  const copied = flash();

  async function copy() {
    try {
      await copyText(command);
      copied.show(true);
    } catch {
      copied.clear();
    }
  }
</script>

<div class="command-row">
  <span class="prompt" aria-hidden="true">$</span>
  <code>{command}</code>
  <button type="button" class="copy" aria-label="Copy {command}" onclick={copy}>{copied.value ? "Copied" : "Copy"}</button>
</div>

<style>
  .command-row {
    display: flex;
    align-items: center;
    gap: calc(10px * var(--ui-scale));
    padding: calc(8px * var(--ui-scale)) calc(12px * var(--ui-scale));
    border: 1px solid var(--border-subtle);
    background: var(--surface-app);
  }

  .command-row + :global(.command-row) {
    border-top: 0;
  }

  .prompt,
  code {
    font: var(--literal);
    font-size: calc(11px * var(--ui-scale));
  }

  .prompt {
    color: var(--text-faint);
  }

  code {
    color: var(--text-secondary);
  }

  .copy {
    margin-left: auto;
    border: 0;
    background: none;
    padding: 0;
    font: var(--machine-xs);
    color: var(--text-faint);
    cursor: pointer;
  }

  .copy:hover,
  .copy:focus-visible {
    color: var(--text-primary);
    outline: none;
  }
</style>
