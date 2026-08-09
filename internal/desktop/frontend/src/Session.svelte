<script>
  import { onMount } from "svelte";
  import Button from "./lib/core/Button.svelte";
  import CapsLabel from "./lib/core/CapsLabel.svelte";
  import Chip from "./lib/core/Chip.svelte";
  import CubeMark from "./lib/core/CubeMark.svelte";
  import StatusDot from "./lib/core/StatusDot.svelte";
  import RailItem from "./lib/session/RailItem.svelte";
  import LatestDocument from "./lib/shell/LatestDocument.svelte";
  import PaneHeader from "./lib/shell/PaneHeader.svelte";
  import TabStrip from "./lib/shell/TabStrip.svelte";
  import TerminalPane from "./lib/shell/TerminalPane.svelte";
  import WindowTray from "./lib/shell/WindowTray.svelte";
  import DockedDocument from "./lib/DockedDocument.svelte";
  import DockedTerminal from "./lib/DockedTerminal.svelte";
  import { age, chrome } from "./lib/chrome.svelte.js";
  import { attach } from "./lib/conversation.js";
  import { closeWindow, openShell, surfaces } from "./lib/docked.svelte.js";

  /** @type {Record<string, {label: string, tone: 'waiting'|'running'|'idle', fill: string}>} */
  const ACTIVITY = {
    waiting: { label: "Waiting for you", tone: "waiting", fill: "var(--state-waiting)" },
    working: { label: "Working", tone: "running", fill: "var(--state-running)" },
    // Idle has no state colour, because idle is not a state worth a hue.
    idle: { label: "Idle", tone: "idle", fill: "var(--ctp-surface-2)" },
  };

  const session = chrome();
  const open = surfaces();
  let host;
  // The focused tab is held by id, so a window docking behind it cannot shift
  // the selection the way an index would.
  let focused = $state("");
  let selected = $derived(Math.max(0, open.tabs.findIndex((tab) => tab.id === focused)));

  onMount(() => attach(host));

  let fields = $derived(session.fields);
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

  <div class="panels">
    <div class="rail">
      <CapsLabel>Sessions</CapsLabel>
      {#each fields.sessions as row (row.slug)}
        <RailItem
          initials={row.initials}
          name={row.name}
          mode={row.mode}
          repos={row.repos}
          live={row.live}
          selected={row.live}
          activity={row.live ? fields.activity : "idle"}
          style="cursor: default" />
      {/each}

      {#if fields.repos.length}
        <div class="repos">
          <CapsLabel tone="dim">This session</CapsLabel>
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
        </div>
      {/if}
    </div>

    <div class="agent">
      <PaneHeader>
        <StatusDot state={activity.tone} />
        <CapsLabel>Agent</CapsLabel>
        <Chip tone={fields.mode === "RPI" ? "guided" : "assistant"} selected>{fields.mode}</Chip>
        {#if fields.phase}<Chip>{fields.phase}</Chip>{/if}
        <span class="badge" class:quiet={fields.activity === "idle"} style:--tone={activity.fill}
          >{activity.label}</span>
        <LatestDocument {latest} count={fields.documents.length} />
      </PaneHeader>
      <TerminalPane>
        <div class="host" bind:this={host}></div>
      </TerminalPane>
    </div>

    <div class="human">
      <TabStrip
        tabs={open.tabs}
        {selected}
        onSelect={(i) => (focused = open.tabs[i].id)}
        onClose={(i) => closeWindow(open.tabs[i].id)}
        onNew={async () => (focused = await openShell())}
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
    border-right: 1px solid var(--border-subtle);
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

  .host {
    flex: 1;
    min-height: 0;
  }

  .human {
    width: var(--w-human-pane);
    flex: none;
    display: flex;
    flex-direction: column;
    background: var(--surface-terminal);
  }
</style>
