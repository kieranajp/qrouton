<script>
  import { dismissible } from "../core/dismiss.js";
  import RoleToggle from "../forms/RoleToggle.svelte";
  import { menuHeight, place } from "../menu.js";
  import Menu from "../shell/Menu.svelte";

  const BASE_MENU_WIDTH = 232;
  const BASE_HEADING = "Branch from";
  const LISTING = "Listing branches…";
  const UNLISTABLE = "Couldn't list branches";

  /** @type {{name?: string, meta?: string, role?: 'off'|'editing'|'reference', offers?: ('off'|'editing'|'reference')[], base?: string, rebasable?: boolean, branches?: {state: 'idle'|'loading'|'ready'|'failed', branches: string[], default: string}, onRoleChange?: (role: string) => void, onBaseChange?: (branch: string) => void, onBaseOpen?: () => void, [attribute: string]: any}} */
  let {
    name,
    meta,
    role = "off",
    offers = ["off", "editing", "reference"],
    base = "",
    rebasable = false,
    branches = { state: "idle", branches: [], default: "" },
    onRoleChange,
    onBaseChange,
    onBaseOpen,
    ...rest
  } = $props();

  let chosen = $derived(role !== "off");
  /** @type {{x: number, y: number} | null} */
  let menu = $state(null);

  // The base a listing has not reached yet is the one the row already shows, so
  // the menu opens on a real answer rather than an empty list.
  let offered = $derived(branches.branches.length ? branches.branches : [base].filter(Boolean));
  let items = $derived([
    { heading: BASE_HEADING },
    ...offered.map((branch) => ({ label: branch, active: branch === base, branch })),
    ...(branches.state === "loading" ? [{ label: LISTING, disabled: true }] : []),
    ...(branches.state === "failed" ? [{ label: UNLISTABLE, disabled: true }] : []),
  ]);

  // The list scrolls, so the menu is drawn at the viewport rather than inside
  // the row it would be clipped in.
  let anchor = $derived(
    menu
      ? place(
          menu,
          { width: BASE_MENU_WIDTH, height: menuHeight(items) },
          { width: window.innerWidth, height: window.innerHeight },
        )
      : null,
  );

  function openMenu(event) {
    const box = event.currentTarget.getBoundingClientRect();
    menu = { x: box.left, y: box.bottom + 4 };
    onBaseOpen?.();
  }
</script>

<div class="row" class:chosen {...rest}>
  <span class="name" class:on={chosen}>{name}</span>
  {#if meta}<span class="meta">{meta}</span>{/if}
  {#if rebasable && base}
    <button class="base" onclick={openMenu} aria-haspopup="menu">
      <span class="branch">{base}</span>
      <span class="caret">▾</span>
    </button>
  {/if}
  <RoleToggle value={role} {offers} onChange={onRoleChange} />
</div>

{#if menu && anchor}
  <div
    class="anchor"
    style:left="{anchor.left}px"
    style:top="{anchor.top}px"
    use:dismissible={() => (menu = null)}>
    <Menu
      {items}
      width={BASE_MENU_WIDTH}
      offsetY={0}
      onSelect={(item) => {
        menu = null;
        if (item.branch) onBaseChange?.(item.branch);
      }} />
  </div>
{/if}

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

  .base {
    display: flex;
    align-items: center;
    gap: 6px;
    max-width: 148px;
    padding: 3px 7px;
    background: transparent;
    border: 1px solid var(--border-default);
    cursor: pointer;
    font: var(--machine-sm);
    font-size: 10.5px;
    color: var(--text-muted);
  }

  .base:hover {
    border-color: var(--accent-action);
    color: var(--text-primary);
  }

  .branch {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .caret {
    flex: none;
    line-height: 1;
  }

  .anchor {
    position: fixed;
    z-index: 5;
  }
</style>
