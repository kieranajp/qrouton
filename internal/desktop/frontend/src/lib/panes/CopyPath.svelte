<script>
  import Button from "../core/Button.svelte";
  import { flash } from "../flash.svelte.js";
  import { copyText } from "../wails.js";

  /** @type {{path?: string}} */
  let { path } = $props();

  /** @type {ReturnType<typeof flash<boolean>>} */
  const copied = flash();

  async function copy() {
    if (!path) return;
    try {
      await copyText(path);
      copied.show(true);
    } catch {
      copied.clear();
    }
  }
</script>

{#if path}
  <Button
    variant="ghost"
    size="sm"
    aria-label="Copy absolute path"
    title={path}
    icon="copy"
    onclick={copy}>{copied.value ? "Copied" : "Copy"}</Button>
{/if}
