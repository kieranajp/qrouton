<script>
  import { onMount } from "svelte";
  import Button from "./lib/core/Button.svelte";
    import CapsLabel from "./lib/core/CapsLabel.svelte";
  import Chip from "./lib/core/Chip.svelte";
  import CubeMark from "./lib/core/CubeMark.svelte";
  import StatusDot from "./lib/core/StatusDot.svelte";
  import Rail from "./lib/session/Rail.svelte";
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
  import { pending as pendingAssembly } from "./lib/assembly/calls.js";
  import { assemblyOpen, pickerOpen } from "./lib/assembly/steps.js";
  import FirstRunOverlay from "./lib/firstrun/FirstRunOverlay.svelte";
  import SettingsOverlay from "./lib/settings/SettingsOverlay.svelte";
  import { age, chrome } from "./lib/chrome.svelte.js";
  import {
    consumeTerminalFocus,
    focusGenerationIn,
    focusPendingIn,
    focusTerminal,
    humanWidth,
    MIN_HUMAN,
    readStored,
    roomFor,
    selectedIn,
    selectIn,
    storedWidth,
    widthKey,
    writeStored,
  } from "./lib/layout.js";
  import {
    closeWindow,
    openDocument,
    openShell,
    selectWindow,
    surfaces,
  } from "./lib/docked.svelte.js";
  import { Events } from "./lib/wails.js";

  /** @type {Record<string, {label: string, tone: 'waiting'|'running'|'idle', fill: string}>} */
  const ACTIVITY = {
    waiting: { label: "Waiting for you", tone: "waiting", fill: "var(--state-waiting)" },
    working: { label: "Working", tone: "running", fill: "var(--state-running)" },
    // Idle has no state colour, because idle is not a state worth a hue.
    idle: { label: "Idle", tone: "idle", fill: "var(--ctp-surface-2)" },
  };

  const session = chrome();
  let fields = $derived(session.fields);
  const open = surfaces(() => fields.slug);
  let dragged = $state({});
  let width = $derived(dragged[fields.slug] ?? storedWidth(readStored, fields.slug));
  let panels = $state(0);
  let rail = $state(0);
  let measured = $state(0);
  let room = $derived(roomFor(panels, rail));
  let human = $derived(humanWidth(width, room));

  function resize(next) {
    dragged = { ...dragged, [fields.slug]: next };
  }

  function commit(next) {
    resize(next);
    writeStored(widthKey(fields.slug), next);
  }

  function reset() {
    resize(0);
    writeStored(widthKey(fields.slug), 0);
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
  // A token the agent terminal watches: bumping it is what hands the keyboard
  // back when whatever covered it goes away — an overlay, or one of the rail's
  // own menus.
  let keyboard = $state(0);
  let requested = $state(false);

  onMount(() => {
    let live = true;
    const off = Events.On("assembly:requested", () => (requested = true));
    pendingAssembly()
      .then((ticket) => {
        if (live && ticket) requested = true;
      })
      .catch(() => {});
    return () => {
      live = false;
      off();
    };
  });

  let assembling = $derived(assemblyOpen(requested, session.settled, fields.slug));
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
  let covered = $derived(fields.welcoming || assembling || picker || settingsOpen);
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
    <Rail
      sessions={fields.sessions}
      slug={fields.slug}
      repos={fields.repos}
      onNewSession={() => (requested = true)}
      onAddRepos={() => (added = fields.slug)}
      onDismissed={() => keyboard++}
      bind:width={rail} />

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
      onCommit={commit}
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

  <WindowTray
    summary="{fields.repos.length} repo{fields.repos.length === 1 ? '' : 's'} · {commits} commit{commits ===
    1
      ? ''
      : 's'}" />

  {#if fields.welcoming}
    <FirstRunOverlay />
  {:else if assembling}
    <Overlay gated={!fields.slug} onClose={() => (requested = false)} />
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
  .human {
    width: var(--w-human-pane);
    flex: none;
    display: flex;
    flex-direction: column;
    background: var(--surface-terminal);
  }
</style>
