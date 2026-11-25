<script>
  import Button from "../core/Button.svelte";
  import { flash } from "../flash.svelte.js";
  import { copyText } from "../wails.js";

  /** @type {{code: string, label?: string}} */
  let { code, label = "command" } = $props();

  /** @type {ReturnType<typeof flash<boolean>>} */
  const copied = flash();

  async function copy() {
    try {
      await copyText(code);
      copied.show(true);
    } catch {
      copied.clear();
    }
  }
</script>

<div class="code" data-copy-code>
  <pre><code>{code}</code></pre>
  <Button variant="ghost" size="sm" aria-label="Copy {label}" title={code} icon="copy" onclick={copy}>{copied.value ? "Copied" : "Copy"}</Button>
</div>

<style>
  .code {
    display: flex;
    align-items: flex-start;
    gap: calc(8px * var(--ui-scale));
    min-width: 0;
  }

  pre {
    flex: 1;
    min-width: 0;
    margin: 0;
    padding: calc(6px * var(--ui-scale)) calc(10px * var(--ui-scale));
    border: var(--border-width) solid var(--border-subtle);
    background: var(--surface-terminal);
    overflow-x: auto;
  }

  code {
    font: var(--machine-sm);
    color: var(--accent-literal);
    white-space: pre;
  }
</style>
