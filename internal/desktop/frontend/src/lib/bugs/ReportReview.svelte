<script>
  import { onMount, untrack } from "svelte";
  import Button from "../core/Button.svelte";
  import { openURL } from "../wails.js";
  import { loadReport, confirmReport, cancelReport } from "./calls.js";

  let { slug, requestID, status, label, onClose } = $props();
  const owner = untrack(() => slug);
  const id = untrack(() => requestID);
  let preview = $state(null);
  let error = $state("");
  let submitting = $state(false);
  let closing = $state(false);
  let panel;
  let live = true;
  let epoch = 0;
  let pending = $derived(preview?.report.status === "pending" && !submitting);

  onMount(() => {
    panel.focus();
    return () => {
      live = false;
      epoch++;
      if (!closing && !submitting && (!preview || preview.report.status === "pending")) {
        cancelReport(owner, id).catch(() => {});
      }
    };
  });

  $effect(() => {
    const observed = status;
    const sequence = ++epoch;
    loadReport(owner, id).then((value) => {
      if (live && sequence === epoch && value.report.id === id) {
        preview = value;
        if (observed !== "pending") submitting = false;
        error = "";
      }
    }).catch((reason) => {
      if (live && sequence === epoch) error = String(reason);
    });
  });

  async function confirm() {
    if (!pending || closing) return;
    submitting = true;
    const sequence = ++epoch;
    try {
      const outcome = await confirmReport(owner, id);
      if (live && sequence === epoch && outcome.id === id) preview = { ...preview, report: outcome };
    } catch (reason) {
      if (live && sequence === epoch) {
        preview = { ...preview, report: { ...preview.report, status: "unknown", message: preview.unknownMessage } };
      }
    }
  }

  async function dismiss() {
    if (closing) return;
    closing = true;
    epoch++;
    try {
      if (!submitting && (!preview || preview.report.status === "pending")) {
        await cancelReport(owner, id);
      }
    } catch (reason) {
      if (live) { error = String(reason); closing = false; }
      return;
    }
    if (live) onClose();
  }

  function keydown(event) {
    if (event.key === "Escape") {
      event.preventDefault();
      event.stopPropagation();
      dismiss();
    }
    if (event.key === "Tab") {
      const buttons = [...panel.querySelectorAll("button:not(:disabled), a[href], [tabindex='0']")];
      const index = buttons.indexOf(document.activeElement);
      if (buttons.length && (index < 0 || (!event.shiftKey && index === buttons.length - 1) || (event.shiftKey && index === 0))) {
        event.preventDefault();
        buttons[event.shiftKey ? buttons.length - 1 : 0].focus();
      }
    }
  }
</script>

<div class="scrim">
  <div class="panel" role="dialog" aria-modal="true" aria-label={label} tabindex="-1" bind:this={panel} onkeydown={keydown}>
    <h2>{label}</h2>
    {#if preview}
      <div class="destination">{preview.destination}</div>
      <!-- svelte-ignore a11y_no_noninteractive_tabindex (Keyboard users must be able to scroll the full report.) -->
      <div class="payload" tabindex="0" role="region" aria-label={preview.title}>
        <h3>{preview.title}</h3>
        <pre>{preview.body}</pre>
      </div>
      <p role="status">{preview.report.message}</p>
      {#if preview.report.url}
        <a href={preview.report.url} onclick={(event) => { event.preventDefault(); openURL(preview.report.url); }}>{preview.report.url}</a>
      {/if}
    {/if}
    {#if error}<p role="alert">{error}</p>{/if}
    <div class="actions">
      <Button variant="secondary" disabled={closing} onclick={dismiss}>{preview ? (pending ? preview.cancelLabel : preview.closeLabel) : "×"}</Button>
      {#if preview?.report.status === "pending"}
        <Button disabled={!pending || closing} onclick={confirm}>{preview.createLabel}</Button>
      {/if}
    </div>
  </div>
</div>

<style>
  .scrim { position: fixed; inset: 0; z-index: 30; display: flex; align-items: center; justify-content: center; background: var(--scrim); }
  .panel { box-sizing: border-box; width: 680px; max-width: calc(100% - 32px); max-height: calc(100% - 32px); display: flex; flex-direction: column; gap: 12px; padding: 18px; background: var(--surface-chrome); color: var(--text-primary); border: 1px solid var(--border-default); box-shadow: var(--shadow-menu); font: var(--machine-sm); }
  h2, h3, p { margin: 0; }
  h2 { font: var(--display-xs); }
  h3 { font: var(--machine-sm); font-weight: bold; overflow-wrap: anywhere; }
  .destination { flex: none; overflow-wrap: anywhere; }
  .payload { min-height: 0; overflow: auto; border: 1px solid var(--border-subtle); padding: 12px; }
  pre { white-space: pre-wrap; overflow-wrap: anywhere; font: inherit; }
  p, a { overflow-wrap: anywhere; flex: none; }
  a { color: var(--accent-action); }
  .actions { display: flex; justify-content: flex-end; gap: 8px; flex: none; }
</style>
