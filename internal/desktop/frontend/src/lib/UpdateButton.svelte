<script>
  import Button from "./core/Button.svelte";
  import { dismissible } from "./core/dismiss.js";
  import { updates } from "./updates.svelte.js";
  import { copyText, openURL } from "./wails.js";

  const view = updates();
  let status = $derived(view.status);
  let open = $state(false);
  let copied = $state(false);

  function click() {
    if (status.command) {
      open = !open;
      return;
    }
    if (status.url) openURL(status.url);
  }

  async function copy() {
    try {
      await copyText(status.command);
      copied = true;
      setTimeout(() => (copied = false), 1200);
    } catch {}
  }
</script>

{#if status.available}
  <div class="anchored" use:dismissible={() => (open = false)}>
    <Button
      variant="primary"
      size="sm"
      title={status.command ? undefined : "Open the release page"}
      onclick={click}>Update to {status.latest}</Button>
    {#if open && status.command}
      <div class="panel">
        <p>
          qrouton <strong>{status.latest}</strong> is out. You have {status.current}. Run this in a
          terminal:
        </p>
        <code>{status.command}</code>
        <Button variant="ghost" size="sm" onclick={copy}>{copied ? "Copied" : "Copy"}</Button>
        {#if status.url}
          <p>
            Downloaded it directly? <a
              href={status.url}
              onclick={(event) => {
                event.preventDefault();
                openURL(status.url);
              }}>Get it from the release page.</a>
          </p>
        {/if}
      </div>
    {/if}
  </div>
{/if}

<style>
  .anchored {
    position: relative;
  }

  .panel {
    position: absolute;
    top: calc(34px * var(--ui-scale));
    right: 0;
    width: calc(260px * var(--ui-scale));
    display: flex;
    flex-direction: column;
    gap: calc(10px * var(--ui-scale));
    padding: calc(12px * var(--ui-scale));
    background: var(--surface-chrome);
    border: 1px solid var(--accent-action);
    box-shadow: var(--shadow-menu);
    z-index: 5;
  }

  .panel p {
    margin: 0;
    font: var(--machine-sm);
    color: var(--text-secondary);
  }

  .panel code {
    font: var(--machine-sm);
    color: var(--text-primary);
    background: var(--surface-raised);
    padding: calc(6px * var(--ui-scale)) calc(8px * var(--ui-scale));
    word-break: break-all;
  }

  .panel a {
    color: var(--accent-action);
  }
</style>
