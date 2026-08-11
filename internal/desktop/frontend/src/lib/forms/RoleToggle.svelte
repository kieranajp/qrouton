<script>
  const ROLES = [
    { key: "off", label: "Off", bg: "var(--surface-raised)", fg: "var(--text-primary)" },
    { key: "editing", label: "Editing", bg: "var(--role-editing)", fg: "var(--text-on-accent)" },
    { key: "reference", label: "Reference", bg: "var(--role-reference)", fg: "var(--text-on-accent)" },
  ];

  /** @type {{value?: 'off'|'editing'|'reference', onChange?: (role: string) => void, [attribute: string]: any}} */
  let { value = "off", onChange, ...rest } = $props();
</script>

<div class="toggle" {...rest}>
  {#each ROLES as role (role.key)}
    <span
      class="role"
      class:on={role.key === value}
      style:--on-bg={role.bg}
      style:--on-fg={role.fg}
      role="presentation"
      onclick={() => onChange?.(role.key)}>{role.label}</span>
  {/each}
</div>

<style>
  .toggle {
    display: flex;
    border: 1px solid var(--border-default);
  }

  .role {
    font: var(--machine-sm);
    font-size: 10.5px;
    color: var(--text-muted);
    background: transparent;
    padding: 4px 10px;
    cursor: pointer;
  }

  .role + .role {
    border-left: 1px solid var(--border-default);
  }

  .on {
    font-weight: 700;
    color: var(--on-fg);
    background: var(--on-bg);
  }
</style>
