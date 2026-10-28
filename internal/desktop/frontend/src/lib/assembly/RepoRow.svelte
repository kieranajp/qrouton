<script>
  import { REPO_ROLES } from "../bridge/generated.js";
  import { dismissible } from "../core/dismiss.js";
  import Icon from "../core/Icon.svelte";
  import TextField from "../forms/TextField.svelte";
  import RoleToggle from "../forms/RoleToggle.svelte";
  import { place } from "../menu.js";
  import { uiScale } from "../scale.svelte.js";
  import Menu from "../shell/Menu.svelte";

  const BASE_MENU_WIDTH = 232;
  const BASE_MENU_MAX_HEIGHT = 300;
  const BASE_HEADING = "Branch from";
  const LISTING = "Listing branches…";
  const UNLISTABLE = "Couldn't list branches";

  /** @type {{name?: string, meta?: string, role?: "off" | import("../bridge/generated.js").RepoRole, offers?: ("off" | import("../bridge/generated.js").RepoRole)[], base?: string, rebasable?: boolean, branches?: {state: 'idle'|'loading'|'ready'|'failed', branches: string[], default: string}, onRoleChange?: (role: string) => void, onBaseChange?: (branch: string) => void, onBaseOpen?: () => void, [attribute: string]: any}} */
  let {
    name,
    meta,
    role = "off",
    offers = ["off", REPO_ROLES.EDITING, REPO_ROLES.REFERENCE],
    base = "",
    rebasable = false,
    branches = { state: "idle", branches: [], default: "" },
    onRoleChange,
    onBaseChange,
    onBaseOpen,
    ...rest
  } = $props();

  let chosen = $derived(role !== "off");
  let query = $state("");
  let trigger = $state(/** @type {HTMLButtonElement | undefined} */ (undefined));
  /** @type {{left: number, top: number, maxHeight: number} | null} */
  let menu = $state(null);

  // The current base stays available before its listing arrives.
  let offered = $derived(branches.branches.length ? branches.branches : [base].filter(Boolean));
  let matches = $derived(offered.filter((branch) => branch.toLowerCase().includes(query.toLowerCase())));
  let items = $derived([
    { heading: BASE_HEADING },
    ...matches.map((branch) => ({ label: branch, active: branch === base, branch })),
    ...(branches.state === "ready" && !matches.length ? [{ label: "No matching branches", disabled: true }] : []),
    ...(branches.state === "loading" ? [{ label: LISTING, disabled: true }] : []),
    ...(branches.state === "failed" ? [{ label: UNLISTABLE, disabled: true }] : []),
  ]);

  function closeMenu() {
    menu = null;
    trigger?.focus();
  }

  $effect(() => {
    if (!menu) return;
    const escape = (event) => {
      if (event.key !== "Escape") return;
      event.preventDefault();
      event.stopPropagation();
      closeMenu();
    };
    window.addEventListener("keydown", escape, true);
    return () => window.removeEventListener("keydown", escape, true);
  });

  function menuKey(event) {
    if (menu && event.key === "Enter") event.stopPropagation();
  }

  function focusSearch(node) {
    node.querySelector("input")?.focus();
  }

  function toggleMenu(event) {
    if (menu) {
      menu = null;
      return;
    }
    query = "";
    const box = event.currentTarget.getBoundingClientRect();
    const scale = uiScale() / 100;
    const maxHeight = Math.min(BASE_MENU_MAX_HEIGHT, (window.innerHeight - 16) / scale);
    // Reserve the capped height so arriving results cannot move or overflow the popup.
    menu = { ...place(
      { x: box.left, y: box.bottom + (4 * uiScale()) / 100 },
      { width: BASE_MENU_WIDTH, height: maxHeight },
      { width: window.innerWidth, height: window.innerHeight },
      scale,
    ), maxHeight };
    onBaseOpen?.();
  }
</script>

<div class="row" class:chosen {...rest}>
  <span class="name" class:on={chosen}>{name}</span>
  {#if meta}<span class="meta">{meta}</span>{/if}
  {#if rebasable && chosen && base}
    <span class="base-control" role="presentation" onkeydown={menuKey} use:dismissible={() => (menu = null)}>
      <button class="base" bind:this={trigger} onclick={toggleMenu} aria-haspopup="menu" aria-expanded={!!menu}>
        <Icon name="branch" />
        <span class="branch">{base}</span>
        <Icon name="menu" />
      </button>
      {#if menu}
        <span class="anchor" style:left="{menu.left}px" style:top="{menu.top}px">
          <Menu
            {items}
            width={BASE_MENU_WIDTH}
            maxHeight={menu.maxHeight}
            offsetY={0}
            onSelect={(item) => {
              closeMenu();
              if (item.branch) onBaseChange?.(item.branch);
            }}>
            {#snippet header()}
              <div use:focusSearch>
                <TextField aria-label="Search branches" placeholder="Search branches" icon="search" valueVoice="literal" bind:value={query} />
              </div>
            {/snippet}
          </Menu>
        </span>
      {/if}
    </span>
  {/if}
  <RoleToggle value={role} {offers} onChange={onRoleChange} />
</div>

<style>
  .row {
    display: grid;
    grid-template-columns: minmax(0, 1fr);
    grid-auto-flow: column;
    grid-auto-columns: max-content;
    align-items: center;
    gap: calc(14px * var(--ui-scale));
    padding: calc(9px * var(--ui-scale)) calc(12px * var(--ui-scale));
    background: transparent;
    border-bottom: 1px solid var(--border-subtle);
  }

  .chosen {
    background: var(--surface-raised);
  }

  .name {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font: var(--machine-md);
    color: var(--text-secondary);
  }

  .name.on {
    font: var(--machine-bold);
    color: var(--text-primary);
  }

  .meta {
    font: var(--machine-sm);
    font-size: calc(10.5px * var(--ui-scale));
    color: var(--text-faint);
  }

  .base-control {
    display: contents;
  }

  .base {
    display: flex;
    align-items: center;
    gap: calc(6px * var(--ui-scale));
    max-width: calc(148px * var(--ui-scale));
    padding: calc(3px * var(--ui-scale)) calc(7px * var(--ui-scale));
    background: transparent;
    border: 1px solid var(--border-default);
    cursor: pointer;
    font: var(--machine-sm);
    font-size: calc(10.5px * var(--ui-scale));
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

  .anchor {
    position: fixed;
    z-index: 5;
  }
</style>
