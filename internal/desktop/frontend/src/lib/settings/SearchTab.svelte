<script>
  import { OLLAMA_STATES } from "../bridge/generated.js";
  import Button from "../core/Button.svelte";
  import ProgressBar from "../core/ProgressBar.svelte";
  import { DOWNLOAD_URL, host, megabytes, ollama, percent } from "../ollama.svelte.js";
  import { openURL } from "../wails.js";
  import CommandRow from "./CommandRow.svelte";
  import StatusLine from "./StatusLine.svelte";
  import TabHeader from "./TabHeader.svelte";

  /** @type {{labelledby?: string}} */
  let { labelledby } = $props();

  const view = ollama();
  let status = $derived(view.status);
</script>

<TabHeader title="Search" id={labelledby}>Find past research, specs and plans by meaning, not exact words.</TabHeader>

<p class="intro">
  Every document in your thoughts folders, shared ones included, is indexed. Agents search the same index, so
  they can find earlier work before starting again. It runs on a small local model through Ollama, so nothing
  leaves your machine.
</p>

{#if status.state === OLLAMA_STATES.READY}
  <StatusLine tone="success">Semantic search is ready.</StatusLine>
{:else if status.state === OLLAMA_STATES.MISSING}
  <StatusLine tone="waiting">
    Ollama is running, but the embedding model is not installed.
    {#snippet actions()}
      <Button variant="cube" onclick={view.pull}>Download model</Button>
      <code>{status.model}</code>
    {/snippet}
  </StatusLine>
{:else if status.state === OLLAMA_STATES.PULLING}
  <StatusLine tone="waiting" detail="Carries on in the background. If qrouton restarts, resume it here.">
    {status.detail || "Downloading"} {status.model}
    {#snippet body()}
      <ProgressBar percent={percent(status)} />
    {/snippet}
    {#snippet actions()}
      {#if status.total}<span class="bytes">{megabytes(status.completed)} of {megabytes(status.total)}</span>{/if}
      <Button variant="secondary" onclick={view.cancel}>Cancel</Button>
    {/snippet}
  </StatusLine>
{:else if status.state === OLLAMA_STATES.UNREACHABLE && status.installed}
  <StatusLine tone="waiting" boxed detail="Nothing answering on {host}">
    Ollama is installed but not running
    {#snippet body()}
      <p class="body">
        Search is keyword-only until it's running. Open the Ollama app, or start the server in a terminal.
      </p>
      <CommandRow command="ollama serve" />
    {/snippet}
    {#snippet actions()}
      <Button variant="cube" onclick={view.check}>Check again</Button>
    {/snippet}
  </StatusLine>
{:else if status.state === OLLAMA_STATES.UNREACHABLE}
  <StatusLine tone="faint" boxed detail="Found no ollama on PATH and nothing on {host}">
    Ollama isn't installed
    {#snippet body()}
      <p class="body">
        Search is keyword-only until it is. Install Ollama, open it once, then come back here to download the
        model.
      </p>
      <div class="commands">
        <CommandRow command="brew install ollama" />
        <div class="download">
          <span>or download from</span>
          <a href={DOWNLOAD_URL} onclick={(event) => (event.preventDefault(), openURL(DOWNLOAD_URL))}
            >ollama.com/download</a>
        </div>
      </div>
    {/snippet}
    {#snippet actions()}
      <Button variant="cube" onclick={view.check}>Check again</Button>
    {/snippet}
  </StatusLine>
{:else if status.state === OLLAMA_STATES.FAILED}
  <StatusLine tone="failed" boxed>
    The download failed: {status.reason}
    {#snippet actions()}
      <Button variant="cube" onclick={view.pull}>Try again</Button>
    {/snippet}
  </StatusLine>
{:else}
  <StatusLine>Checking Ollama…</StatusLine>
{/if}

<style>
  .intro,
  .body {
    margin: 0;
    max-width: calc(640px * var(--ui-scale));
    font: var(--machine-sm);
    font-size: calc(11px * var(--ui-scale));
    color: var(--text-muted);
    text-wrap: pretty;
  }

  code,
  .bytes {
    font: var(--machine-sm);
    color: var(--text-primary);
  }

  .commands {
    display: flex;
    flex-direction: column;
  }

  .download {
    display: flex;
    align-items: center;
    gap: calc(10px * var(--ui-scale));
    padding: calc(8px * var(--ui-scale)) calc(12px * var(--ui-scale));
    border: 1px solid var(--border-subtle);
    border-top: 0;
    background: var(--surface-app);
    font: var(--machine-xs);
    color: var(--text-faint);
  }

  a {
    font: var(--literal);
    font-size: calc(11px * var(--ui-scale));
    color: var(--accent-action);
    text-decoration: none;
  }

  a:hover {
    color: var(--ctp-lavender);
  }
</style>
