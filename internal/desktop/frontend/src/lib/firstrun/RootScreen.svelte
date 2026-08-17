<script>
  import Button from "../core/Button.svelte";
  import CapsLabel from "../core/CapsLabel.svelte";
  import TextField from "../forms/TextField.svelte";

  /** @type {{root?: string, error?: string, onChoose?: () => void}} */
  let { root = $bindable(""), error = "", onChoose } = $props();
</script>

<h1>Where should sessions live?</h1>

<p>
  One folder holds every session and the shared mirrors. Nothing is written outside it, and deleting
  a session only removes its worktrees — the mirrors stay for next time.
</p>

<TextField bind:value={root} valueVoice="literal">
  {#snippet trailing()}
    <Button variant="secondary" onclick={onChoose}>Choose…</Button>
  {/snippet}
</TextField>

{#if error}
  <p class="help failed">{error}</p>
{:else}
  <p class="help">
    Default is <span class="path">~/work</span>. This folder will be created if it does not exist.
  </p>
{/if}

<div class="everything">
  <CapsLabel>That is everything</CapsLabel>
  <p>
    Next you will pick repositories for your first session. Editors and agent commands have sensible
    defaults — Settings has them when you want them.
  </p>
</div>

<style>
  h1 {
    margin: 0;
    font: var(--display-md);
    letter-spacing: var(--display-tracking);
    color: var(--text-primary);
  }

  p {
    margin: 0;
    font: var(--machine-md);
    color: var(--text-secondary);
    max-width: 78ch;
  }

  .help {
    font: var(--machine-sm);
  }

  .help.failed {
    color: var(--state-failed);
  }

  .path {
    color: var(--text-primary);
  }

  .everything {
    display: flex;
    flex-direction: column;
    gap: 8px;
    padding: 14px 16px;
    border: 1px solid var(--border-default);
  }

  .everything p {
    font: var(--machine-sm);
  }
</style>
