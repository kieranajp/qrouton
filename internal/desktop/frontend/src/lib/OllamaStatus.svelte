<script>
  import { onMount } from "svelte";
  import { OLLAMA_STATES } from "./bridge/generated.js";
  import Button from "./core/Button.svelte";
  import { ollama } from "./ollama.svelte.js";
  import { openURL } from "./wails.js";

  const DOWNLOAD_URL = "https://ollama.com/download";

  const view = ollama();
  onMount(view.check);
  let status = $derived(view.status);
  let percent = $derived(status.total ? Math.round((status.completed / status.total) * 100) : 0);

  /** @param {number} [bytes] */
  const mb = (bytes = 0) => `${(bytes / 1e6).toFixed(1)} MB`;
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
    <div class="bar" role="progressbar" aria-valuenow={percent} aria-valuemin="0" aria-valuemax="100">
      <span style:width="{percent}%"></span>
    </div>
    <div class="actions">
      {#if status.total}<span class="bytes">{mb(status.completed)} of {mb(status.total)}</span>{/if}
      <Button variant="secondary" onclick={view.cancel}>Cancel</Button>
    </div>
    <p class="quiet">Carries on in the background. If qrouton restarts, resume it in Settings.</p>
  {:else if status.state === OLLAMA_STATES.UNREACHABLE && status.installed}
    <p>Ollama is installed but not running. Start it, then check again.</p>
    <div class="actions">
      <Button variant="secondary" onclick={view.check}>Check again</Button>
    </div>
  {:else if status.state === OLLAMA_STATES.UNREACHABLE}
    <p>Ollama is not installed, so search will be keyword-only until it is.</p>
    <div class="actions">
      <Button variant="secondary" onclick={() => openURL(DOWNLOAD_URL)}>Get Ollama</Button>
      <Button variant="ghost" onclick={view.check}>Check again</Button>
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
    gap: 10px;
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
    gap: 12px;
  }

  code,
  .bytes {
    font: var(--machine-sm);
    color: var(--text-primary);
  }

  .bar {
    height: 4px;
    background: var(--border-subtle);
  }

  .bar span {
    display: block;
    height: 100%;
    background: var(--accent-action);
  }
</style>
