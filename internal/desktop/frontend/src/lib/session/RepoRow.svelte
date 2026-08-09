<script>
  import RoleToggle from "../forms/RoleToggle.svelte";

  /** @type {{name?: string, pushed?: string, role?: 'off'|'editing'|'reference', onRoleChange?: (role: string) => void, [attribute: string]: any}} */
  let { name, pushed, role = "off", onRoleChange, ...rest } = $props();
</script>

<div class="row" class:editing={role === "editing"} {...rest}>
  <span class="name" class:on={role !== "off"}>{name}</span>
  {#if pushed}<span class="pushed">pushed {pushed}</span>{/if}
  <RoleToggle value={role} onChange={onRoleChange} />
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

  .editing {
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

  .pushed {
    font: var(--machine-sm);
    font-size: 10.5px;
    color: var(--text-faint);
  }
</style>
