<script>
  import OllamaStatus from "../OllamaStatus.svelte";
  import CapsLabel from "../core/CapsLabel.svelte";
</script>

<h1>Search by meaning as well?</h1>

<p>
  With Ollama and a small embedding model, thoughts search finds documents by meaning as well as by
  their words. Without it, search is keyword-only and everything else works the same.
</p>

<OllamaStatus />

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
