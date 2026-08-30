<script>
  import Button from "../core/Button.svelte";
  import { copyText } from "../wails.js";

  /** @type {{path?: string}} */
  let { path } = $props();

  let copied = $state(false);

  async function copy() {
    if (!path) return;
    try {
      await copyText(path);
      copied = true;
      setTimeout(() => (copied = false), 1200);
    } catch {}
  }
</script>

{#if path}
  <Button
    variant="ghost"
    size="sm"
    aria-label="Copy absolute path"
    title={path}
    onclick={copy}>{copied ? "Copied" : "Copy"}</Button>
{/if}
