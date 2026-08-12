<script>
  import { onMount } from "svelte";
  import Button from "../core/Button.svelte";
  import Stepper from "../forms/Stepper.svelte";
  import { intent } from "./steps.js";

  /** @type {{steps?: string[], active?: number, secondary: string, primary: string, status?: string, busy?: boolean, onSecondary: () => void, onPrimary: () => void, onEscape: () => void, children: import("svelte").Snippet}} */
  let {
    steps = [],
    active = 0,
    secondary,
    primary,
    status = "",
    busy = false,
    onSecondary,
    onPrimary,
    onEscape,
    children,
  } = $props();

  let layer = $state(/** @type {HTMLElement | undefined} */ (undefined));

  // Focus starts on the layer, so Escape and Enter work before a field is touched.
  onMount(() => layer?.focus());

  // Escape is the layer's own key rather than a dismissible action, which closes
  // on a press outside the node too — and the backdrop of a full-window layer is
  // outside, so one stray click would throw a half-filled form away.
  function key(event) {
    const meant = intent(event);
    if (!meant) return;
    event.preventDefault();
    if (meant === "cancel") onEscape();
    else onPrimary();
  }
</script>

<div class="layer" role="presentation" tabindex="-1" bind:this={layer} onkeydown={key}>
  <div class="dialog">
    {#if steps.length}
      <div class="steps">
        <Stepper {steps} {active} />
      </div>
    {/if}

    <div class="body">
      {@render children()}
    </div>

    <div class="footer">
      <Button variant="secondary" disabled={busy} onclick={onSecondary}>{secondary}</Button>
      <div class="advance">
        <span class="status">{status}</span>
        <Button disabled={busy} onclick={onPrimary}>{primary}</Button>
      </div>
    </div>
  </div>
</div>

<style>
  /* Above the app's own ceiling of 5: over the rail, both panes and the tray. */
  .layer {
    position: fixed;
    inset: 0;
    z-index: 10;
    display: flex;
    align-items: center;
    justify-content: center;
    background: var(--surface-canvas);
    outline: none;
  }

  .dialog {
    width: 1100px;
    max-width: 100%;
    min-height: 470px;
    max-height: 100%;
    display: flex;
    flex-direction: column;
    background: var(--surface-app);
    border: 1px solid var(--border-subtle);
    overflow: hidden;
  }

  .steps {
    flex: none;
    padding: 22px 34px 0;
  }

  .body {
    flex: 1;
    min-height: 0;
    display: flex;
    flex-direction: column;
    gap: 18px;
    padding: 22px 34px 30px;
    overflow: hidden auto;
  }

  .footer {
    height: var(--h-form-footer);
    flex: none;
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 0 34px;
    background: var(--surface-chrome);
    border-top: 1px solid var(--border-subtle);
  }

  .advance {
    display: flex;
    align-items: center;
    gap: 14px;
  }

  .status {
    font: var(--machine-sm);
    font-size: 11.5px;
    color: var(--text-muted);
  }
</style>
