<script>
  import Button from "../core/Button.svelte";
  import CapsLabel from "../core/CapsLabel.svelte";
  import TextField from "../forms/TextField.svelte";

  /** @type {{thoughts?: string, derived?: string, error?: string, onChoose?: () => void}} */
  let { thoughts = $bindable(""), derived = "", error = "", onChoose } = $props();
</script>

<h1>Where should thoughts go?</h1>

<p>
  Each session writes its research, specs and plans here as plain Markdown, so Obsidian or any
  editor can open the folder. An existing Obsidian vault works too.
</p>

<TextField bind:value={thoughts} valueVoice="literal">
  {#snippet trailing()}
    <Button variant="secondary" onclick={onChoose}>Choose…</Button>
  {/snippet}
</TextField>

{#if error}
  <p class="help failed">{error}</p>
{:else}
  <p class="help">
    Default is <span class="path">{derived}</span>, inside the sessions folder.
  </p>
{/if}
<p class="help">Folders shared with a team, such as one Syncthing syncs, are set up in Settings.</p>

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
