<script>
  import RoleToggle from "../forms/RoleToggle.svelte";

  /** @type {{name?: string, meta?: string, role?: 'off'|'editing'|'reference', locked?: boolean, onRoleChange?: (role: string) => void, [attribute: string]: any}} */
  let { name, meta, role = "off", locked = false, onRoleChange, ...rest } = $props();

  let chosen = $derived(role !== "off");
</script>

<div class="row" class:chosen {...rest}>
  <span class="name" class:on={chosen}>{name}</span>
  {#if meta}<span class="meta">{meta}</span>{/if}
  <RoleToggle value={role} disabled={locked} onChange={onRoleChange} />
</div>

<style>
  .row {
    display: flex;
    align-items: center;
    gap: 14px;
    padding: 9px 12px;
    background: transparent;
    border-bottom: 1px solid var(--border-subtle);
  }

  .chosen {
    background: var(--surface-raised);
  }

  .name {
    flex: 1;
    font: var(--machine-md);
    color: var(--text-secondary);
  }

  .name.on {
    font: var(--machine-bold);
    color: var(--text-primary);
  }

  .meta {
    font: var(--machine-sm);
    font-size: 10.5px;
    color: var(--text-faint);
  }
</style>
