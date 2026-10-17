<script>
  import { onMount } from "svelte";
  import { OLLAMA_STATES } from "./bridge/generated.js";
  import Button from "./core/Button.svelte";
  import ProgressBar from "./core/ProgressBar.svelte";
  import { DOWNLOAD_URL, megabytes, ollama, percent } from "./ollama.svelte.js";
  import { openURL } from "./wails.js";

  const view = ollama();
  onMount(view.check);
  let status = $derived(view.status);
</script>

<div class="ollama">
  {#if status.state === OLLAMA_STATES.READY}
    <p class="quiet">Semantic search is ready.</p>
  {:else if status.state === OLLAMA_STATES.MISSING}
    <p>Ollama is running, but the embedding model is not installed.</p>
    <div class="actions">
      <Button variant="secondary" onclick={view.pull}>Download model</Button>
      <code>{status.model}</code>
    </div>
  {:else if status.state === OLLAMA_STATES.PULLING}
    <p>{status.detail || "Downloading"} {status.model}</p>
    <ProgressBar percent={percent(status)} />
    <div class="actions">
      {#if status.total}<span class="bytes">{megabytes(status.completed)} of {megabytes(status.total)}</span>{/if}
      <Button variant="secondary" onclick={view.cancel}>Cancel</Button>
    </div>
    <p class="quiet">Carries on in the background. If qrouton restarts, resume it in Settings.</p>
  {:else if status.state === OLLAMA_STATES.UNREACHABLE && status.installed}
    <p>Ollama is installed but not running. Start it, then check again.</p>
    <div class="actions">
      <Button variant="secondary" icon="refresh" onclick={view.check}>Check again</Button>
    </div>
  {:else if status.state === OLLAMA_STATES.UNREACHABLE}
    <p>Ollama is not installed, so search will be keyword-only until it is.</p>
    <div class="actions">
      <Button variant="secondary" onclick={() => openURL(DOWNLOAD_URL)}>Get Ollama</Button>
      <Button variant="ghost" icon="refresh" onclick={view.check}>Check again</Button>
    </div>
  {:else if status.state === OLLAMA_STATES.FAILED}
    <p class="failed">The download failed: {status.reason}</p>
    <div class="actions">
      <Button variant="secondary" onclick={view.pull}>Try again</Button>
    </div>
  {:else}
    <p class="quiet">Checking Ollama…</p>
  {/if}
</div>

<style>
  .ollama {
    display: flex;
    flex-direction: column;
    gap: calc(10px * var(--ui-scale));
  }

  p {
    margin: 0;
    font: var(--machine-sm);
    color: var(--text-secondary);
  }

  .quiet {
    color: var(--text-muted);
  }

  .failed {
    color: var(--state-failed);
  }

  .actions {
    display: flex;
    align-items: center;
    gap: calc(12px * var(--ui-scale));
  }

  code,
  .bytes {
    font: var(--machine-sm);
    color: var(--text-primary);
  }
</style>
