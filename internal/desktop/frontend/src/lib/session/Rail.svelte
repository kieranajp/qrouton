<script>
  import { onMount, onDestroy } from "svelte";
  import Button from "../core/Button.svelte";
  import CapsLabel from "../core/CapsLabel.svelte";
  import Icon from "../core/Icon.svelte";
  import { dismissible } from "../core/dismiss.js";
  import { flash } from "../flash.svelte.js";
  import Confirm from "../shell/Confirm.svelte";
  import Menu from "../shell/Menu.svelte";
  import RailItem from "./RailItem.svelte";
  import { fitMenu } from "../menu.js";
  import { cleanup, cycleSticker, reload, reorder, reveal, show, showDashboard, switchProfile, uncommitted, unload } from "../sessions.js";
  import { DASHBOARD_TITLE } from "../dashboard/dashboard.js";
  import {
    NO_CONVERSATION_REFUSAL,
    PROFILE_CURRENT_META,
    PROFILE_DEFAULT_LABEL,
    SESSION_REORDER_FAILED,
    SESSION_SHOW_FAILED,
    SESSION_SWITCH_PROFILE_FAILED,
    SESSION_SWITCH_PROFILE_LABEL,
    SESSION_SWITCH_PROFILE_NEEDS_MESSAGE,
    SESSION_UNLOAD_FAILED,
    SESSION_UNLOAD_LABEL,
  } from "../bridge/generated.js";
  import { age } from "../relative.js";
  import { rowAt, shortcut } from "../shortcuts.js";
  import { stickerFeedback } from "./stickers.js";

  /** @type {{sessions: any[], slug: string, stickers: import("./stickers.js").Sticker[], profiles?: {runner: string, profiles: {id: string, label: string}[]}[], onNewSession: () => void, onOwnership?: (value: boolean) => void, onDismissed: () => void, size: number, width: number}} */
  let {
    sessions,
    slug,
    stickers = [],
    profiles = [],
    onNewSession,
    onOwnership,
    onDismissed,
    size = 0,
    // The splitter needs to know how much room the rail leaves the panes, and
    // only the rail knows how wide it drew itself.
    width = $bindable(0),
  } = $props();

  const ROW_MENU_WIDTH = 190;

  let dragSlug = $state("");
  let targetSlug = $state("");
  /** @type {HTMLDivElement} */
  let sessionList;
  let gesture = $state(
    /** @type {{pointerId: number, source: string, button: HTMLElement, startX: number, startY: number, x: number, y: number, moved: boolean} | null} */ (null),
  );
  let scrollFrame = 0;

  $effect(() => {
    const source = gesture?.source;
    if ((source && !sessions.some((row) => row.slug === source)) ||
        (targetSlug && !sessions.some((row) => row.slug === targetSlug))) {
      clearDrag();
    }
  });

  function clearDrag() {
    const previous = gesture;
    gesture = null;
    dragSlug = "";
    targetSlug = "";
    cancelAnimationFrame(scrollFrame);
    scrollFrame = 0;
    if (previous?.button.hasPointerCapture(previous.pointerId)) {
      previous.button.releasePointerCapture(previous.pointerId);
    }
  }

  function startPointer(event, source) {
    event.preventDefault();
    if (event.button !== 0 || !event.isPrimary || gesture) return;
    gesture = {
      pointerId: event.pointerId,
      source,
      button: event.currentTarget,
      startX: event.clientX,
      startY: event.clientY,
      x: event.clientX,
      y: event.clientY,
      moved: false,
    };
    event.currentTarget.setPointerCapture(event.pointerId);
  }

  function findTarget() {
    if (!gesture?.moved) return;
    const row = document.elementFromPoint(gesture.x, gesture.y)?.closest("[data-session-slug]");
    const slug = row && sessionList.contains(row) ? row.getAttribute("data-session-slug") : "";
    targetSlug = slug && slug !== gesture.source && sessions.some((row) => row.slug === slug) ? slug : "";
  }

  function scrollDrag() {
    if (!gesture?.moved) return;
    const box = sessionList.getBoundingClientRect();
    if (gesture.x >= box.left && gesture.x <= box.right &&
        gesture.y >= box.top && gesture.y <= box.bottom) {
      const edge = 28;
      const top = gesture.y - box.top;
      const bottom = box.bottom - gesture.y;
      const delta = top < edge ? -(edge - top) / 2 : bottom < edge ? (edge - bottom) / 2 : 0;
      sessionList.scrollTop += delta;
      findTarget();
    }
    scrollFrame = requestAnimationFrame(scrollDrag);
  }

  function movePointer(event) {
    if (!gesture || event.pointerId !== gesture.pointerId) return;
    event.preventDefault();
    gesture.x = event.clientX;
    gesture.y = event.clientY;
    if (!gesture.moved && Math.hypot(gesture.x - gesture.startX, gesture.y - gesture.startY) >= 5) {
      gesture.moved = true;
      dragSlug = gesture.source;
      scrollFrame = requestAnimationFrame(scrollDrag);
    }
    findTarget();
  }

  async function finishPointer(event) {
    if (!gesture || event.pointerId !== gesture.pointerId) return;
    movePointer(event);
    const { source, moved } = gesture;
    const target = targetSlug;
    const sourceExists = sessions.some((row) => row.slug === source);
    const valid = sourceExists && target && sessions.some((row) => row.slug === target);
    clearDrag();
    if (!moved) {
      if (sourceExists) openSession(source);
      return;
    }
    if (!valid) return;
    try {
      await reorder(source, target);
    } catch {
      showRowNotice(source, SESSION_REORDER_FAILED, true);
    }
  }

  function cancelPointer(event) {
    if (gesture?.pointerId === event.pointerId) clearDrag();
  }

  const stickerQueues = new Map();
  let pendingStickers = $state({});
  /** @type {ReturnType<typeof flash<{slug: string, sequence: number, text: string, failed: boolean}>>} */
  const rowNotice = flash();
  let noticeSequence = 0;
  let mounted = true;

  onMount(() => {
    const onKey = (event) => {
      if (event.key === "Escape" && gesture) {
        event.preventDefault();
        clearDrag();
        return;
      }
      const row = rowAt(sessions, event);
      if (!row) return;
      event.preventDefault();
      openSession(row.slug);
    };
    window.addEventListener("keydown", onKey);
    window.addEventListener("blur", clearDrag);
    return () => {
      mounted = false;
      window.removeEventListener("keydown", onKey);
      window.removeEventListener("blur", clearDrag);
      clearDrag();
    };
  });

  export async function openSession(slug) {
    try {
      await show(slug);
    } catch (err) {
      const reason = String(err?.message ?? "").trim();
      showRowNotice(slug, reason ? `${SESSION_SHOW_FAILED}: ${reason}` : SESSION_SHOW_FAILED, true);
    }
  }

  function showRowNotice(slug, text, failed = false) {
    if (!mounted) return;
    rowNotice.show({ slug, text, failed, sequence: ++noticeSequence });
  }

  function changeSticker(event, row) {
    event.stopPropagation();
    const slug = row.slug;
    pendingStickers = { ...pendingStickers, [slug]: (pendingStickers[slug] ?? 0) + 1 };

    const previous = stickerQueues.get(slug) ?? Promise.resolve();
    const queued = previous
      .catch(() => {})
      .then(() => cycleSticker(slug))
      .then((committed) => showRowNotice(slug, stickerFeedback(committed, stickers)))
      .catch(() => showRowNotice(slug, "Sticker could not be changed", true))
      .finally(() => {
        pendingStickers = {
          ...pendingStickers,
          [slug]: Math.max(0, (pendingStickers[slug] ?? 1) - 1),
        };
        if (stickerQueues.get(slug) === queued) stickerQueues.delete(slug);
      });
    stickerQueues.set(slug, queued);
  }

  let menu = $state(/** @type {{row: any, x: number, y: number} | null} */ (null));
  /** @type {import("../shell/menu.js").MenuItem[]} */
  const rowMenu = $derived([
    { label: "Reload", icon: "refresh", act: reloadRow },
    ...profileMenu(sessions.find((row) => row.slug === menu?.row.slug)),
    {
      label: SESSION_UNLOAD_LABEL,
      icon: "close",
      act: unloadRow,
      disabled: !sessions.find((row) => row.slug === menu?.row.slug)?.terminal,
    },
    { label: "Reveal in Finder", icon: "worktree", act: revealRow },
    "-",
    { label: "Clean up…", icon: "bin", tone: "destructive", act: askCleanup },
  ]);
  /** @returns {import("../shell/menu.js").MenuItem[]} */
  function profileMenu(row) {
    const named = profiles.find((group) => group.runner === row?.runner)?.profiles ?? [];
    if (!named.length) return [];
    const choices = [{ id: "", label: PROFILE_DEFAULT_LABEL }, ...named];
    return [{
      label: SESSION_SWITCH_PROFILE_LABEL,
      icon: "agent",
      items: choices.map((profile) => {
        const current = profile.id === (row.profile ?? "");
        return {
          label: profile.label,
          meta: current ? PROFILE_CURRENT_META : undefined,
          active: current,
          disabled: current,
          act: (picked) => switchRow(picked, profile.id),
        };
      }),
    }];
  }
  let confirming = $state(/** @type {{row: any, dirty: string[], checked: boolean, error: string} | null} */ (null));
  let cleaning = $state(false);
  let inspecting = $state(false);
  $effect(() => { onOwnership?.(Boolean(menu || confirming || cleaning || inspecting || gesture)); });
  onDestroy(() => onOwnership?.(false));

  // The rail clips and scrolls, so the menu is drawn at the page and placed at
  // the pointer instead of anchored to the row.
  function openMenu(event, row) {
    // The webview draws one of its own otherwise.
    event.preventDefault();
    menu = { row, x: event.clientX, y: event.clientY };
  }

  function dismissMenu() {
    menu = null;
    onDismissed();
  }

  // A supervisor this workbench does not own is refused rather than killed, and
  // the row is where that refusal has to land.
  async function reloadRow(row) {
    onDismissed();
    try {
      await reload(row.slug);
    } catch {
      showRowNotice(row.slug, "Session could not be reloaded", true);
    }
  }

  async function switchRow(row, profile) {
    onDismissed();
    try {
      await switchProfile(row.slug, profile);
    } catch (err) {
      const unrecorded = String(err?.message ?? err).includes(NO_CONVERSATION_REFUSAL);
      showRowNotice(row.slug, unrecorded ? SESSION_SWITCH_PROFILE_NEEDS_MESSAGE : SESSION_SWITCH_PROFILE_FAILED, true);
    }
  }

  async function unloadRow(row) {
    onDismissed();
    try {
      await unload(row.slug);
    } catch {
      showRowNotice(row.slug, SESSION_UNLOAD_FAILED, true);
    }
  }

  // Nothing here changed, so a Finder that will not open is left silent.
  async function revealRow(row) {
    onDismissed();
    try {
      await reveal(row.slug);
    } catch {}
  }

  // The list is advisory — the removal forces regardless — so a check that could
  // not be made says so and leaves the confirm live.
  async function askCleanup(row) {
    inspecting = true;
    let dirty = [];
    let checked = true;
    try {
      dirty = await uncommitted(row.slug);
    } catch {
      checked = false;
    }
    confirming = { row, dirty, checked, error: "" };
    inspecting = false;
  }

  // The chrome poll notices the row is gone, so nothing here waits on a refresh.
  async function cleanUp() {
    const current = confirming;
    if (cleaning || !current) return;
    cleaning = true;
    try {
      await cleanup(current.row.slug);
      dismissConfirm();
    } catch (err) {
      confirming = { ...current, error: String(err?.message ?? err) };
    } finally {
      cleaning = false;
    }
  }

  function dismissConfirm() {
    confirming = null;
    onDismissed();
  }
</script>

<div class="rail" style:width={size ? size + "px" : null} bind:clientWidth={width}>
  <!-- Outside the session list, so a drag can neither start here nor drop here. -->
  <div class="home">
    <button
      type="button"
      class="dashboard"
      class:selected={!slug}
      aria-current={!slug ? "page" : undefined}
      onclick={() => showDashboard()}>
      <span class="tile"><Icon name="dashboard" inherit /></span>
      <span class="label">{DASHBOARD_TITLE}</span>
    </button>
  </div>

  <div class="band">
    <CapsLabel tone="dim" centred>Sessions</CapsLabel>
  </div>

  <div class="session-list" aria-label="Sessions" bind:this={sessionList}>
    {#each sessions as row, i (row.slug)}
      <RailItem
        slug={row.slug}
        initials={row.initials}
        shortcut={shortcut(i)}
        name={row.name}
        repos={row.repos}
        summary={row.summary}
        idle={row.summary?.running ? "" : age(row.opened)}
        selected={row.slug === slug}
        dragging={row.slug === dragSlug}
        over={row.slug === targetSlug}
        onPointerDown={(event) => startPointer(event, row.slug)}
        onPointerMove={movePointer}
        onPointerUp={finishPointer}
        onPointerCancel={cancelPointer}
        unseen={row.unseen}
        stickerId={row.sticker}
        {stickers}
        stickerBusy={(pendingStickers[row.slug] ?? 0) > 0}
        feedback={rowNotice.value?.slug === row.slug ? rowNotice.value : null}
        onSelect={() => openSession(row.slug)}
        onSticker={(event) => changeSticker(event, row)}
        onContextMenu={(event) => openMenu(event, row)} />
    {/each}
  </div>

  <div class="footer">
    <Button variant="cube" size="sm" icon="new" wide onclick={onNewSession}>New session</Button>
  </div>
</div>

{#if menu}
  {@const row = menu.row}
  <div
    class="anchor"
    use:fitMenu={menu}
    use:dismissible={dismissMenu}>
    <Menu
      items={rowMenu}
      width={ROW_MENU_WIDTH}
      offsetY={0}
      onSelect={(item) => {
        const picked = row;
        menu = null;
        item.act(picked);
      }} />
  </div>
{/if}

{#if confirming}
  <Confirm
    title="Clean up {confirming.row.name}?"
    confirmLabel="Clean up"
    busy={cleaning}
    onConfirm={cleanUp}
    onCancel={dismissConfirm}>
    <div class="tell">
      {#if confirming.error}
        <p class="lost">{confirming.error}</p>
      {:else}
        <p>Removes its worktrees and session files.</p>
        <p>
          Kept: the shared mirrors, this session's branch inside them, and the documents under
          thoughts/.
        </p>
        {#if !confirming.checked}
          <p>Could not check for uncommitted work.</p>
        {:else if confirming.dirty.length}
          <p class="lost">Uncommitted files will be lost in:</p>
          <ul class="lost">
            {#each confirming.dirty as repo (repo)}
              <li>{repo}</li>
            {/each}
          </ul>
        {/if}
      {/if}
    </div>
  </Confirm>
{/if}

<style>
  .rail {
    width: var(--w-rail);
    box-sizing: border-box;
    flex: none;
    display: flex;
    flex-direction: column;
    background: var(--surface-chrome);
    overflow: hidden;
  }

  /* A pane header's height, so the Dashboard row shares a line with the pane headers beside it. */
  .home {
    height: var(--h-pane-header);
    box-sizing: border-box;
    flex: none;
    display: flex;
    align-items: center;
    padding: 0 calc(12px * var(--ui-scale));
  }

  .dashboard {
    width: 100%;
    display: flex;
    align-items: center;
    gap: calc(10px * var(--ui-scale));
    padding: calc(6px * var(--ui-scale)) calc(8px * var(--ui-scale));
    border: 1px solid transparent;
    background: transparent;
    color: var(--text-muted);
    font: var(--machine-md);
    font-size: calc(11px * var(--ui-scale));
    text-align: left;
    cursor: pointer;
  }

  .dashboard:hover:not(.selected) {
    border-color: var(--border-default);
    background: var(--surface-raised);
    color: var(--text-primary);
  }

  .dashboard.selected {
    border-color: var(--accent-action);
    background: var(--surface-raised);
    box-shadow: var(--shadow-focus);
    font: var(--machine-bold);
    font-size: calc(11px * var(--ui-scale));
    color: var(--text-primary);
  }

  .dashboard:focus-visible {
    outline: 2px solid var(--accent-action);
    outline-offset: -2px;
  }

  .tile {
    width: calc(30px * var(--ui-scale));
    height: calc(30px * var(--ui-scale));
    flex: none;
    display: flex;
    align-items: center;
    justify-content: center;
    border: 1px solid var(--border-subtle);
    color: var(--text-faint);
  }

  .dashboard.selected .tile {
    background: var(--accent-action);
    border-color: var(--accent-action);
    color: var(--text-on-accent);
  }

  .band {
    flex: none;
    display: flex;
    align-items: center;
    justify-content: center;
    padding: calc(12px * var(--ui-scale)) calc(12px * var(--ui-scale)) calc(8px * var(--ui-scale));
  }

  /* The session list scrolls; the footer below it stays put. */
  .session-list {
    flex: 1 1 auto;
    min-height: 0;
    overflow: hidden auto;
    display: flex;
    flex-direction: column;
    gap: calc(4px * var(--ui-scale));
    padding: 0 calc(12px * var(--ui-scale));
  }

  .footer {
    flex: none;
    display: flex;
    flex-direction: column;
    border-top: 1px solid var(--border-subtle);
    padding: calc(12px * var(--ui-scale));
  }

  .anchor {
    position: fixed;
    width: 0;
    height: 0;
    z-index: 20;
  }

  .tell {
    display: flex;
    flex-direction: column;
    gap: calc(9px * var(--ui-scale));
    line-height: 1.5;
  }

  .tell p,
  .tell ul {
    margin: 0;
  }

  .tell ul {
    padding-left: calc(18px * var(--ui-scale));
  }

  .lost {
    color: var(--action-destructive);
  }
</style>
