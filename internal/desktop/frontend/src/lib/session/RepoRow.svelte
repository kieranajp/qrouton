<script>
  import { dismissible } from "../core/dismiss.js";
  import RoleToggle from "../forms/RoleToggle.svelte";
  import { menuHeight, place } from "../menu.js";
  import Menu from "../shell/Menu.svelte";

  const BASE_MENU_WIDTH = 232;
  const BASE_MENU_MAX_HEIGHT = 300;
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
  /** @type {{left: number, top: number} | null} */
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
  // the row it would be clipped in. Placed once on opening: a menu that moved
  // when its branches arrived would shift what the pointer is already over.
  function toggleMenu(event) {
    if (menu) {
      menu = null;
      return;
    }
    const box = event.currentTarget.getBoundingClientRect();
    menu = place(
      { x: box.left, y: box.bottom + 4 },
      { width: BASE_MENU_WIDTH, height: Math.min(menuHeight(items), BASE_MENU_MAX_HEIGHT) },
      { width: window.innerWidth, height: window.innerHeight },
    );
    onBaseOpen?.();
  }
</script>

<div class="row" class:chosen {...rest}>
  <span class="name" class:on={chosen}>{name}</span>
  {#if meta}<span class="meta">{meta}</span>{/if}
  {#if rebasable && chosen && base}
    <span class="base-control" use:dismissible={() => (menu = null)}>
      <button class="base" onclick={toggleMenu} aria-haspopup="menu" aria-expanded={!!menu}>
        <span class="branch">{base}</span>
        <span class="caret">▾</span>
      </button>
      {#if menu}
        <span class="anchor" style:left="{menu.left}px" style:top="{menu.top}px">
          <Menu
            {items}
            width={BASE_MENU_WIDTH}
            maxHeight={BASE_MENU_MAX_HEIGHT}
            offsetY={0}
            onSelect={(item) => {
              menu = null;
              if (item.branch) onBaseChange?.(item.branch);
            }} />
        </span>
      {/if}
    </span>
  {/if}
  <RoleToggle value={role} {offers} onChange={onRoleChange} />
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

  .base-control {
    display: contents;
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
