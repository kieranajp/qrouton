<script>
  import { onMount } from "svelte";
  import Dialog from "../assembly/Dialog.svelte";
  import Button from "../core/Button.svelte";
  import Icon from "../core/Icon.svelte";
  import Settings from "./Settings.svelte";
  import { settings } from "./settings.svelte.js";

  /** @type {{onClose: () => void}} */
  let { onClose } = $props();

  const panel = settings(() => onClose());

  // A Dialog focuses its layer on mount alone, so the one underneath needs it back.
  onMount(() => {
    const beneath = /** @type {HTMLElement | null} */ (document.activeElement);
    return () => beneath?.focus?.();
  });
</script>

<Dialog
  secondary="Cancel"
  primary="Save"
  status={panel.status}
  busy={panel.saving}
  height
  flush
  onSecondary={panel.cancel}
  onPrimary={panel.save}
  onEscape={panel.cancel}>
  <Settings {panel} />
  {#snippet footer()}
    {#if panel.restartRequired}
      <span class="restart"><Icon name="refresh" />Quit qrouton to use the new sessions root and thoughts folders</span>
      <Button variant="cube" onclick={panel.quitAndRelaunch}>Quit qrouton</Button>
    {/if}
  {/snippet}
</Dialog>

<style>
  .restart {
    display: inline-flex;
    align-items: center;
    gap: calc(6px * var(--ui-scale));
    font: var(--machine-sm);
    color: var(--state-waiting);
  }
</style>
