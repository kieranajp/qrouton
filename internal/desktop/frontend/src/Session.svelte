<script>
  import { onMount } from "svelte";
  import Button from "./lib/core/Button.svelte";
  import CapsLabel from "./lib/core/CapsLabel.svelte";
  import Chip from "./lib/core/Chip.svelte";
  import CubeMark from "./lib/core/CubeMark.svelte";
  import StatusDot from "./lib/core/StatusDot.svelte";
  import RailItem from "./lib/session/RailItem.svelte";
  import Confirm from "./lib/shell/Confirm.svelte";
  import LatestDocument from "./lib/shell/LatestDocument.svelte";
  import Menu from "./lib/shell/Menu.svelte";
  import PaneHeader from "./lib/shell/PaneHeader.svelte";
  import Splitter from "./lib/shell/Splitter.svelte";
  import TabStrip from "./lib/shell/TabStrip.svelte";
  import WindowTray from "./lib/shell/WindowTray.svelte";
  import DockedDocument from "./lib/DockedDocument.svelte";
  import DockedTerminal from "./lib/DockedTerminal.svelte";
  import SessionTerminal from "./lib/SessionTerminal.svelte";
  import Overlay from "./lib/assembly/Overlay.svelte";
  import PickerOverlay from "./lib/assembly/PickerOverlay.svelte";
  import { pickerOpen } from "./lib/assembly/steps.js";
  import { dismissible } from "./lib/core/dismiss.js";
  import SettingsOverlay from "./lib/settings/SettingsOverlay.svelte";
  import { age, chrome } from "./lib/chrome.svelte.js";
  import {
    consumeTerminalFocus,
    focusGenerationIn,
    focusPendingIn,
    focusTerminal,
    selectedIn,
    selectIn,
    storedWidth,
    widthKey,
  } from "./lib/layout.js";
  import { menuHeight, place } from "./lib/menu.js";
  import { cleanup, reveal, show, uncommitted } from "./lib/sessions.js";
  import { rowAt, shortcut } from "./lib/shortcuts.js";
  import {
    closeWindow,
    openDocument,
    openShell,
    selectWindow,
    surfaces,
  } from "./lib/docked.svelte.js";

  /** @type {Record<string, {label: string, tone: 'waiting'|'running'|'idle', fill: string}>} */
  const ACTIVITY = {
    waiting: { label: "Waiting for you", tone: "waiting", fill: "var(--state-waiting)" },
    working: { label: "Working", tone: "running", fill: "var(--state-running)" },
    // Idle has no state colour, because idle is not a state worth a hue.
    idle: { label: "Idle", tone: "idle", fill: "var(--ctp-surface-2)" },
  };

  // Neither pane is worth having below these; the divider stops rather than
  // letting one of them become a strip.
  const MIN_HUMAN = 320;
  const MIN_AGENT = 360;

  // A page served from a custom scheme has an origin the webview may call
  // opaque, where storage throws rather than coming back empty.
  const readStored = (key) => {
    try {
      return localStorage.getItem(key);
    } catch {
      return null;
    }
  };
  const writeStored = (key, value) => {
    try {
      if (value) localStorage.setItem(key, String(value));
      else localStorage.removeItem(key);
    } catch {}
  };

  const session = chrome();
  let fields = $derived(session.fields);
  const open = surfaces(() => fields.slug);
  let dragged = $state({});
  let width = $derived(dragged[fields.slug] ?? storedWidth(readStored, fields.slug));
  let panels = $state(0);
  let rail = $state(0);
  let measured = $state(0);
  let room = $derived(panels ? Math.max(MIN_HUMAN, panels - rail - MIN_AGENT) : Infinity);
  let human = $derived(width ? Math.min(Math.max(width, MIN_HUMAN), room) : 0);

  function resize(next) {
    dragged = { ...dragged, [fields.slug]: next };
    writeStored(widthKey(fields.slug), next);
  }

  function reset() {
    resize(0);
  }

  let selection = $state({});
  let applied = $state({});
  let terminalFocus = $state({});
  let selectedID = $derived(selectedIn(selection, fields.slug));
  let selected = $derived(Math.max(0, open.tabs.findIndex((tab) => tab.id === selectedID)));

  $effect(() => {
    const slug = fields.slug;
    const id = open.selected;
    if (selectedIn(applied, slug) === id) return;
    applied = selectIn(applied, slug, id);
    selection = selectIn(selection, slug, id);
  });

  const showTab = (id) => (selection = selectIn(selection, fields.slug, id));

  function userSelect(tab) {
    showTab(tab.id);
    if (tab.kind === "terminal") terminalFocus = focusTerminal(terminalFocus, tab.id);
    selectWindow(fields.slug, tab.id).catch(() => {});
  }

  function terminalFocused(id, generation) {
    terminalFocus = consumeTerminalFocus(terminalFocus, id, generation);
  }

  // The session on screen may have no rail row yet: a session is named by
  // adopting it, which happens once the window is already up.
  let mounted = $derived([
    ...(fields.terminal ? [{ terminal: fields.terminal, slug: fields.slug }] : []),
    ...fields.sessions.filter((row) => row.terminal && row.terminal !== fields.terminal),
  ]);

  // A rail row's number is its position, so it moves when showing a session
  // re-sorts the list.
  onMount(() => {
    const onKey = (event) => {
      const row = rowAt(fields.sessions, event);
      if (!row) return;
      event.preventDefault();
      show(row.slug);
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  });

  let activity = $derived(ACTIVITY[fields.activity] ?? ACTIVITY.idle);
  let latest = $derived(
    fields.documents.length
      ? {
          tag: fields.documents[0].kind,
          name: fields.documents[0].name,
          age: age(fields.documents[0].at),
        }
      : undefined,
  );
  let written = $derived(
    fields.documents.map((doc) => ({ tag: doc.kind, label: doc.name, meta: age(doc.at) })),
  );
  let listing = $state(false);

  // A session with no editor has nothing to open and no room here to say so.
  async function read(path) {
    listing = false;
    try {
      showTab(await openDocument(path));
    } catch {}
  }
  const ROW_MENU = [
    { label: "Reveal in Finder", act: revealRow },
    "-",
    { label: "Clean up…", tone: "destructive", act: askCleanup },
  ];
  const ROW_MENU_WIDTH = 190;

  /** @type {{row: any, x: number, y: number} | null} */
  let menu = $state(null);
  /** @type {{row: any, dirty: string[], checked: boolean, error: string} | null} */
  let confirming = $state(null);
  let cleaning = $state(false);
  // A token the terminal watches: bumping it is what hands the keyboard back
  // when an overlay closes.
  let keyboard = $state(0);

  // The rail clips and scrolls, so the menu is drawn at the page and placed at
  // the pointer instead of anchored to the row.
  let anchor = $derived(
    menu
      ? place(
          menu,
          { width: ROW_MENU_WIDTH, height: menuHeight(ROW_MENU) },
          { width: window.innerWidth, height: window.innerHeight },
        )
      : null,
  );

  function openMenu(event, row) {
    // The webview draws one of its own otherwise.
    event.preventDefault();
    menu = { row, x: event.clientX, y: event.clientY };
  }

  function dismissMenu() {
    menu = null;
    keyboard++;
  }

  // Nothing here changed, and the rail has no surface to report a refusal on.
  async function revealRow(row) {
    keyboard++;
    try {
      await reveal(row.slug);
    } catch {}
  }

  // The list is advisory — the removal forces regardless — so a check that could
  // not be made says so and leaves the confirm live.
  async function askCleanup(row) {
    let dirty = [];
    let checked = true;
    try {
      dirty = await uncommitted(row.slug);
    } catch {
      checked = false;
    }
    confirming = { row, dirty, checked, error: "" };
  }

  // The chrome poll notices the row is gone, so nothing here waits on a refresh.
  async function cleanUp() {
    if (cleaning) return;
    cleaning = true;
    try {
      await cleanup(confirming.row.slug);
      dismissConfirm();
    } catch (err) {
      confirming = { ...confirming, error: String(err?.message ?? err) };
    } finally {
      cleaning = false;
    }
  }

  function dismissConfirm() {
    confirming = null;
    keyboard++;
  }

  let assembling = $state(false);
  let settingsOpen = $state(false);
  // An escalation is the shown session's own pending request, so switching
  // session takes its picker with it. Add-repos is this page's, and belongs to
  // the session it was pressed on for the same reason.
  let added = $state("");
  let picker = $derived(pickerOpen(fields.slug, fields.picker, added));

  // An overlay covering the terminal hands the keyboard back when it goes, the
  // same way the rail's menu and its confirm do. Watching the state rather than
  // the dismissal catches the picker the backend closes, which no handler here
  // ever sees.
  let covered = $derived(assembling || picker || settingsOpen);
  let wasCovered = false;
  $effect(() => {
    if (wasCovered && !covered) keyboard++;
    wasCovered = covered;
  });

  let commits = $derived(
    fields.repos.reduce((total, repo) => (repo.measured === false ? total : total + repo.commits), 0),
  );
</script>

<div class="session">
  <div class="titlebar">
    <CubeMark size={20} />
    <span class="name">{fields.identity}</span>
    {#if fields.branch}<span class="branch">{fields.branch}</span>{/if}
    <span class="tools">
      <Button variant="ghost" size="sm" onclick={() => (settingsOpen = true)}>Settings</Button>
    </span>
  </div>

  <div class="panels" bind:clientWidth={panels}>
    <div class="rail" bind:clientWidth={rail}>
      <CapsLabel>Sessions</CapsLabel>
      {#each fields.sessions as row, i (row.slug)}
        <RailItem
          initials={row.initials}
          shortcut={shortcut(i)}
          name={row.name}
          repos={row.repos}
          live={!!row.terminal}
          selected={row.slug === fields.slug}
          activity={row.activity}
          unseen={row.unseen}
          onclick={() => show(row.slug)}
          oncontextmenu={(event) => openMenu(event, row)} />
      {/each}

      <Button variant="dashed" size="sm" glyph="+" onclick={() => (assembling = true)}
        >New session</Button>

      <div class="repos">
        <CapsLabel tone="dim">This session</CapsLabel>
        {#if !fields.repos.length}
          <div class="empty">no repositories yet</div>
        {/if}
        {#each fields.repos as repo (repo.name)}
          <div class="repo">
            <div class="repo-name {repo.role}">
              {repo.role === "editing" ? "●" : "◐"}
              {repo.name}
            </div>
            <div class="repo-stat">
              {#if repo.role === "reference"}
                read-only
              {:else if repo.measured === false}
                unmeasured
              {:else}
                {repo.commits} commit{repo.commits === 1 ? "" : "s"}
                {#if repo.insertions || repo.deletions}
                  · <span class="added">+{repo.insertions}</span>
                  <span class="removed">−{repo.deletions}</span>
                {/if}
              {/if}
            </div>
          </div>
        {/each}
        <Button
          variant="dashed"
          size="sm"
          glyph="+"
          onclick={() => (added = fields.slug)}
          style="margin-top: 6px">Add repos</Button>
      </div>
    </div>

    <div class="agent">
      <PaneHeader>
        <StatusDot state={activity.tone} />
        <CapsLabel>Agent</CapsLabel>
        <Chip tone={fields.mode === "RPI" ? "guided" : "assistant"} selected>{fields.mode}</Chip>
        {#if fields.phase}<Chip>{fields.phase}</Chip>{/if}
        <span class="badge" class:quiet={fields.activity === "idle"} style:--tone={activity.fill}
          >{activity.label}</span>
        <LatestDocument
          {latest}
          count={fields.documents.length}
          open={listing}
          onToggle={() => (listing = !listing)}>
          <Menu
            label="Written this session"
            items={written}
            align="right"
            width={320}
            onSelect={(_, i) => read(fields.documents[i].path)} />
        </LatestDocument>
      </PaneHeader>
      {#each mounted as row (row.terminal)}
        <SessionTerminal
          id={row.terminal}
          active={row.terminal === fields.terminal}
          focus={keyboard} />
      {/each}
    </div>

    <Splitter
      size={human || measured}
      min={MIN_HUMAN}
      max={room}
      onResize={resize}
      onReset={reset}
      label="Resize the shell pane" />

    <div class="human" style:width={human ? human + "px" : null} bind:clientWidth={measured}>
      <TabStrip
        tabs={open.tabs}
        {selected}
        onSelect={(i) => userSelect(open.tabs[i])}
        onClose={(i) => closeWindow(open.tabs[i].id)}
        onNew={async () => userSelect({ id: await openShell(), kind: "terminal" })}
        newLabel="Shell" />
      {#each open.tabs as tab, i (tab.id)}
        <!-- Only a terminal may be Started; a document tab has no process behind it. -->
        {#if tab.kind === "terminal"}
          <DockedTerminal
            id={tab.id}
            active={i === selected}
            focus={focusGenerationIn(terminalFocus, tab.id)}
            focusPending={focusPendingIn(terminalFocus, tab.id)}
            onFocused={(generation) => terminalFocused(tab.id, generation)} />
        {:else}
          <DockedDocument id={tab.id} active={i === selected} />
        {/if}
      {/each}
    </div>
  </div>

  {#if menu}
    <div
      class="anchor"
      style:left="{anchor.left}px"
      style:top="{anchor.top}px"
      use:dismissible={dismissMenu}>
      <Menu
        items={ROW_MENU}
        width={ROW_MENU_WIDTH}
        offsetY={0}
        onSelect={(item) => {
          const row = menu.row;
          menu = null;
          item.act(row);
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

  <WindowTray
    summary="{fields.repos.length} repo{fields.repos.length === 1 ? '' : 's'} · {commits} commit{commits ===
    1
      ? ''
      : 's'}" />

  {#if assembling}
    <Overlay onClose={() => (assembling = false)} />
  {:else if picker}
    <!-- Keyed on the session, so arriving at another one draws that session's
         picker rather than keeping this one over it. -->
    {#key fields.slug}
      <PickerOverlay slug={fields.slug} onClose={() => (added = "")} />
    {/key}
  {:else if settingsOpen}
    <SettingsOverlay onClose={() => (settingsOpen = false)} />
  {/if}
</div>

<style>
  .session {
    height: 100%;
    display: flex;
    flex-direction: column;
    background: var(--surface-app);
    overflow: hidden;
  }

  /* No traffic lights: on Linux the compositor draws the decorations, and
     painting fake ones is a button that does nothing. */
  .titlebar {
    height: var(--h-titlebar);
    flex: none;
    display: flex;
    align-items: center;
    gap: 14px;
    padding: 0 14px;
    background: var(--surface-chrome);
    border-bottom: 1px solid var(--border-subtle);
    user-select: none;
  }

  .name {
    font: var(--display-xs);
    letter-spacing: -0.01em;
    color: var(--text-primary);
  }

  .branch {
    font: var(--machine-sm);
    font-size: 11px;
    color: var(--text-muted);
  }

  .tools {
    margin-left: auto;
    display: flex;
    gap: 8px;
  }

  .panels {
    flex: 1;
    min-height: 0;
    display: flex;
  }

  .rail {
    width: var(--w-rail);
    flex: none;
    display: flex;
    flex-direction: column;
    gap: 8px;
    padding: 14px 12px;
    background: var(--surface-chrome);
    border-right: 1px solid var(--border-subtle);
    overflow: hidden auto;
  }

  .repos {
    margin-top: auto;
    padding-top: 10px;
    border-top: 1px solid var(--border-subtle);
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .repo {
    display: flex;
    flex-direction: column;
    gap: 2px;
    padding: 5px 0;
  }

  .empty {
    font: var(--machine-xs);
    font-size: 10.5px;
    color: var(--text-faint);
    padding: 5px 0;
  }

  .repo-name {
    font: var(--machine-xs);
    font-size: 10.5px;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .repo-name.editing {
    color: var(--role-editing);
  }

  .repo-name.reference {
    color: var(--role-reference);
  }

  .repo-stat {
    font: var(--machine-xs);
    font-size: 9.5px;
    color: var(--text-faint);
  }

  .added {
    color: var(--state-success);
  }

  .removed {
    color: var(--state-failed);
  }

  .agent {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
  }

  .badge {
    font: var(--machine-bold);
    font-size: 10.5px;
    color: var(--text-on-accent);
    background: var(--tone);
    padding: 2px 8px;
  }

  .badge.quiet {
    color: var(--text-secondary);
  }

  /* A zero-size point for the menu to resolve its own position against. */
  .anchor {
    position: fixed;
    width: 0;
    height: 0;
    z-index: 20;
  }

  .tell {
    display: flex;
    flex-direction: column;
    gap: 9px;
    line-height: 1.5;
  }

  .tell p,
  .tell ul {
    margin: 0;
  }

  .tell ul {
    padding-left: 18px;
  }

  .lost {
    color: var(--action-destructive);
  }

  .human {
    width: var(--w-human-pane);
    flex: none;
    display: flex;
    flex-direction: column;
    background: var(--surface-terminal);
  }
</style>
