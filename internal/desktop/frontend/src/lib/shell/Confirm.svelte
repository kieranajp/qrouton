<script>
  import Button from "../core/Button.svelte";
  import { dismissible } from "../core/dismiss.js";

  /** @type {{title: string, confirmLabel?: string, onConfirm?: () => void, onCancel?: () => void, children?: import('svelte').Snippet}} */
  let { title, confirmLabel = "Confirm", onConfirm, onCancel, children } = $props();

  let actions = $state();
  // The dialog takes the keyboard so Enter and Escape do not reach the agent's
  // terminal, and Enter has to mean the thing the dialog was opened to ask.
  $effect(() => {
    /** @type {HTMLButtonElement | null | undefined} */ (
      actions?.querySelector("[data-confirm]")
    )?.focus();
  });
</script>

<div class="scrim">
  <div class="panel" role="dialog" aria-modal="true" aria-label={title} use:dismissible={() => onCancel?.()}>
    <div class="title">{title}</div>
    <div class="body">{@render children?.()}</div>
    <div class="actions" bind:this={actions}>
      <Button variant="secondary" onclick={() => onCancel?.()}>Cancel</Button>
      <Button variant="destructive" data-confirm="true" onclick={() => onConfirm?.()}
        >{confirmLabel}</Button>
    </div>
  </div>
</div>

<style>
  .scrim {
    position: fixed;
    inset: 0;
    z-index: 30;
    display: flex;
    align-items: center;
    justify-content: center;
    background: rgb(0 0 0 / 0.5);
  }

  .panel {
    width: 420px;
    max-width: calc(100% - 40px);
    display: flex;
    flex-direction: column;
    gap: 12px;
    padding: 18px;
    background: var(--surface-chrome);
    border: 1px solid var(--border-default);
    box-shadow: var(--shadow-menu);
  }

  .title {
    font: var(--display-xs);
    color: var(--text-primary);
  }

  .body {
    font: var(--machine-sm);
    color: var(--text-secondary);
  }

  .actions {
    display: flex;
    justify-content: flex-end;
    gap: 8px;
  }
</style>
