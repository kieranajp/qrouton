<script>
  import CapsLabel from "../core/CapsLabel.svelte";

  // An agent window never steals focus, so this line is the whole attention
  // mechanism: tag, name and age, never a bare count.
  /** @type {{latest?: {tag: string, name: string, age: string}, count?: number, open?: boolean, unseen?: boolean, onToggle?: () => void, children?: import('svelte').Snippet, [attribute: string]: any}} */
  let { latest, count = 0, open = false, unseen = false, onToggle, children, ...rest } = $props();
</script>

<div class="latest" {...rest}>
  <CapsLabel tone="dim">Wrote</CapsLabel>
  {#if !latest}
    <span class="nothing">nothing yet</span>
  {:else}
    {#snippet summary()}
      <span class="tag" class:plan={latest.tag === "PLAN"}>{latest.tag}</span>
      <span class="name">{latest.name}</span>
      <span class="age">{latest.age}</span>
      {#if count > 1}<span class="age">+{count - 1}</span>{/if}
    {/snippet}
    {#if onToggle}
      <button class="chip" class:lit={open || unseen} onclick={onToggle}>
        {@render summary()}
        <span class="caret">&#9662;</span>
      </button>
    {:else}
      <span class="chip inert" class:lit={unseen}>{@render summary()}</span>
    {/if}
    {#if open}{@render children?.()}{/if}
  {/if}
</div>

<style>
  .latest {
    display: flex;
    align-items: center;
    gap: 9px;
    position: relative;
  }

  .nothing {
    font: var(--machine-sm);
    color: var(--text-faint);
  }

  .chip {
    display: flex;
    align-items: center;
    gap: 8px;
    background: var(--surface-raised);
    color: var(--text-primary);
    border: 1px solid var(--border-default);
    border-radius: 0;
    font: var(--machine-sm);
    padding: 4px 8px 4px 4px;
    cursor: pointer;
  }

  .lit {
    border-color: var(--accent-action);
  }

  /* A readout, not a control: no caret promising a menu, no cursor promising a
     press. */
  .chip.inert {
    cursor: default;
  }

  .tag {
    font: 700 9px var(--font-machine);
    color: var(--text-on-accent);
    background: var(--accent-label);
    padding: 2px 6px;
  }

  .tag.plan {
    background: var(--state-guided);
  }

  .name {
    max-width: 180px;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .age {
    color: var(--text-faint);
    font: var(--machine-xs);
    font-size: 10px;
  }

  .caret {
    color: var(--text-muted);
  }
</style>
