<script>
  import { onMount } from "svelte";
  import Button from "./lib/core/Button.svelte";
  import CapsLabel from "./lib/core/CapsLabel.svelte";
  import Chip from "./lib/core/Chip.svelte";
  import CubeMark from "./lib/core/CubeMark.svelte";
  import StatusDot from "./lib/core/StatusDot.svelte";
  import RailItem from "./lib/session/RailItem.svelte";
  import LatestDocument from "./lib/shell/LatestDocument.svelte";
  import Menu from "./lib/shell/Menu.svelte";
  import PaneHeader from "./lib/shell/PaneHeader.svelte";
  import Splitter from "./lib/shell/Splitter.svelte";
  import TabStrip from "./lib/shell/TabStrip.svelte";
  import WindowTray from "./lib/shell/WindowTray.svelte";
  import DockedDocument from "./lib/DockedDocument.svelte";
  import DockedTerminal from "./lib/DockedTerminal.svelte";
  import SessionTerminal from "./lib/SessionTerminal.svelte";
  import { age, chrome } from "./lib/chrome.svelte.js";
  import { focusedIn, focusIn, storedWidth, widthKey } from "./lib/layout.js";
  import { show } from "./lib/sessions.js";
  import { position, shortcut } from "./lib/shortcuts.js";
  import {
    closeWindow,
    openDocument,
    openOnboard,
    openPicker,
    openShell,
    surfaces,
    whenSelected,
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

  let focus = $state({});
  let focused = $derived(focusedIn(focus, fields.slug));
  let selected = $derived(Math.max(0, open.tabs.findIndex((tab) => tab.id === focused)));
  const select = (id) => (focus = focusIn(focus, fields.slug, id));

  // The session on screen may have no rail row yet: onboarding names the session
  // it assembles only once the window is already up.
  let mounted = $derived([
    ...(fields.terminal ? [{ terminal: fields.terminal, slug: fields.slug }] : []),
    ...fields.sessions.filter((row) => row.terminal && row.terminal !== fields.terminal),
  ]);

  onMount(() => whenSelected(() => fields.slug, select));

  // A rail row's number is its position, so it moves when showing a session
  // re-sorts the list. Nothing here reaches a floating window's own page.
  onMount(() => {
    const onKey = (event) => {
      const at = position(event);
      const row = at && fields.sessions[at - 1];
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
      select(await openDocument(path));
    } catch {}
  }
  // The picker writes the manifest and the chrome poll notices, so nothing here
  // waits on it.
  async function addRepos() {
    try {
      select(await openPicker());
    } catch {}
  }
  // Onboarding names the session it assembles by adopting it, and the workbench
  // switches to it then; nothing here waits.
  async function newSession() {
    try {
      select(await openOnboard());
    } catch {}
  }
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
      <Button variant="ghost" size="sm" disabled>Settings</Button>
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
          onclick={() => show(row.slug)} />
      {/each}

      <Button variant="dashed" size="sm" glyph="+" onclick={newSession}>New session</Button>

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
        <Button variant="dashed" size="sm" glyph="+" onclick={addRepos} style="margin-top: 6px"
          >Add repos</Button>
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
        <SessionTerminal id={row.terminal} active={row.terminal === fields.terminal} />
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
        onSelect={(i) => select(open.tabs[i].id)}
        onClose={(i) => closeWindow(open.tabs[i].id)}
        onNew={async () => select(await openShell())}
        newLabel="Shell" />
      {#each open.tabs as tab, i (tab.id)}
        <!-- Only a terminal may be Started; a document tab has no process behind it. -->
        {#if tab.kind === "terminal"}
          <DockedTerminal id={tab.id} active={i === selected} />
        {:else}
          <DockedDocument id={tab.id} active={i === selected} />
        {/if}
      {/each}
    </div>
  </div>

  <WindowTray
    windows={open.floating}
    onClose={(window) => closeWindow(window.id)}
    right="{fields.repos.length} repo{fields.repos.length === 1 ? '' : 's'} · {commits} commit{commits ===
    1
      ? ''
      : 's'}" />
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

  .human {
    width: var(--w-human-pane);
    flex: none;
    display: flex;
    flex-direction: column;
    background: var(--surface-terminal);
  }
</style>
