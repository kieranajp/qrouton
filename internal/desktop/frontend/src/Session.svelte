<script>
  import Button from "./lib/core/Button.svelte";
  import CapsLabel from "./lib/core/CapsLabel.svelte";
  import CubeMark from "./lib/core/CubeMark.svelte";
  import { dismissible } from "./lib/core/dismiss.js";
  import Rail from "./lib/session/Rail.svelte";
  import Terminal from "./lib/session/Terminal.svelte";
  import ContextMenu from "./lib/shell/ContextMenu.svelte";
  import DocumentIndex from "./lib/shell/DocumentIndex.svelte";
  import Menu from "./lib/shell/Menu.svelte";
  import PaneHeader from "./lib/shell/PaneHeader.svelte";
  import StageMarks from "./lib/shell/StageMarks.svelte";
  import Splitter from "./lib/shell/Splitter.svelte";
  import TabStrip from "./lib/shell/TabStrip.svelte";
  import DockedDocument from "./lib/DockedDocument.svelte";
  import Overlay from "./lib/assembly/Overlay.svelte";
  import PickerOverlay from "./lib/assembly/PickerOverlay.svelte";
  import FirstRunOverlay from "./lib/firstrun/FirstRunOverlay.svelte";
  import SettingsOverlay from "./lib/settings/SettingsOverlay.svelte";
  import { conversationPTY, tabPTY } from "./lib/session/services.js";
  import { shell } from "./lib/session/shell.svelte.js";
  import { MAX_SIDEBAR, MIN_HUMAN, MIN_SIDEBAR } from "./lib/layout.js";

  const view = shell();
  let fields = $derived(view.fields);
</script>

<div class="session">
  <div class="titlebar">
    <div class="identity">
      <CubeMark size={20} />
      <div class="anchored" use:dismissible={() => (view.identityOpen = false)}>
        <button
          class="name"
          aria-expanded={view.identityOpen}
          onclick={() => (view.identityOpen = !view.identityOpen)}>
          <span class="said">{fields.identity}</span>
          <span class="caret" aria-hidden="true">&#9662;</span>
        </button>
        {#if view.identityOpen}
          <Menu
            items={view.identityMenu}
            width={280}
            offsetY={34}
            onSelect={(item) => view.chose(item)} />
        {/if}
      </div>
    </div>
    <span class="tools">
      <Button variant="ghost" size="sm" onclick={() => (view.settingsOpen = true)}>Settings</Button>
    </span>
  </div>

  <div class="panels" bind:clientWidth={view.panels}>
    <Rail
      sessions={fields.sessions}
      slug={fields.slug}
      repos={fields.repos}
      agents={fields.agents}
      stickerLabels={fields.stickerLabels}
      onNewSession={() => (view.requested = true)}
      onAddRepos={() => (view.added = fields.slug)}
      onDismissed={view.handBack}
      size={view.sidebarSize}
      bind:width={view.railMeasured} />

    <Splitter
      size={view.rail || MIN_SIDEBAR}
      min={MIN_SIDEBAR}
      max={MAX_SIDEBAR}
      side="left"
      onResize={view.resizeSidebar}
      onCommit={view.commitSidebar}
      onReset={view.resetSidebar}
      label="Resize the sidebar" />

    <div class="agent">
      <PaneHeader>
        {#snippet lead()}
          {#if fields.mode === "ASSISTANT"}
            <span class="assistant-mode">Assistant</span>
            <Button
              variant="cube"
              size="sm"
              disabled={view.escalating}
              onclick={view.escalate}>Escalate</Button>
          {:else}
            <StageMarks stages={fields.stages} />
          {/if}
        {/snippet}
        <CapsLabel tone="dim">Agent</CapsLabel>
        {#snippet actions()}
          <DocumentIndex
            count={fields.documents.length}
            open={view.listing}
            onToggle={view.hasDocuments ? () => (view.listing = !view.listing) : undefined}>
            <Menu
              label="Written this session"
              items={view.documentMenu}
              align="right"
              width={320}
              onSelect={(item) => item.path && view.read(item.path)} />
          </DocumentIndex>
        {/snippet}
      </PaneHeader>
      {#each view.conversations as row (row.terminal)}
        <Terminal
          id={row.terminal}
          pty={conversationPTY}
          active={row.terminal === fields.terminal}
          focus={view.focusOf(row.terminal)}
          focusPending={view.focusPendingOf(row.terminal)}
          onFocused={(generation) => view.focused(row.terminal, generation)} />
      {/each}
    </div>

    <Splitter
      size={view.human || view.measured}
      min={MIN_HUMAN}
      max={view.room}
      onResize={view.resize}
      onCommit={view.commit}
      onReset={view.reset}
      label="Resize the shell pane" />

    <div
      class="human"
      style:width={view.human ? view.human + "px" : null}
      bind:clientWidth={view.measured}>
      <TabStrip
        tabs={view.tabs}
        selected={view.selected}
        onSelect={(i) => view.select(view.tabs[i])}
        onClose={(i) => view.close(view.tabs[i])}
        onReorder={view.reorder}
        onNew={view.newShell}
        newLabel="Shell" />
      {#each view.tabs as tab, i (tab.id)}
        <!-- Only a terminal may be Started; a document tab has no process behind it. -->
        {#if tab.kind === "terminal"}
          <Terminal
            id={tab.id}
            pty={tabPTY}
            active={i === view.selected}
            focus={view.focusOf(tab.id)}
            focusPending={view.focusPendingOf(tab.id)}
            onFocused={(generation) => view.focused(tab.id, generation)} />
        {:else}
          <DockedDocument id={tab.id} active={i === view.selected} />
        {/if}
      {/each}
    </div>
  </div>

  <ContextMenu />

  {#if fields.welcoming}
    <FirstRunOverlay />
  {:else if view.assembling}
    <Overlay gated={!fields.slug} onClose={() => (view.requested = false)} />
  {:else if view.picker}
    <!-- Keyed on the session, so arriving at another one draws that session's
         picker rather than keeping this one over it. -->
    {#key fields.slug}
      <PickerOverlay
        slug={fields.slug}
        escalating={fields.picker}
        onClose={() => (view.added = "")} />
    {/key}
  {/if}

  <!-- Stacked rather than branched: unmounting the assembly overlay ends its draft. -->
  {#if view.settingsOpen}
    <SettingsOverlay onClose={() => (view.settingsOpen = false)} />
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

  /* macOS draws its real traffic lights over the left of this band, so that
     strip is theirs and the rest of it drags the window. */
  .titlebar {
    height: var(--h-titlebar);
    flex: none;
    position: relative;
    display: flex;
    align-items: center;
    padding: 0 14px 0 var(--w-traffic-lights);
    background: var(--surface-chrome);
    border-bottom: 1px solid var(--border-subtle);
    user-select: none;
    --wails-draggable: drag;
    /* Above the panes, so the menu the session name opens is not painted over by
       a pane header or a tab strip, each of which stacks itself above the pane. */
    z-index: 6;
  }

  /* Centred on the window rather than on the room the lights and the tools
     leave, so the name does not shift when either changes width. Capped at 60%
     of it, so a long name truncates instead of pushing the group off centre. */
  .identity {
    position: absolute;
    left: 50%;
    transform: translateX(-50%);
    display: flex;
    align-items: center;
    gap: 10px;
    max-width: 60%;
    min-width: 0;
    --wails-draggable: no-drag;
  }

  .anchored {
    position: relative;
    display: flex;
    align-items: center;
    min-width: 0;
  }

  .name {
    display: flex;
    align-items: center;
    gap: 8px;
    min-width: 0;
    max-width: 460px;
    padding: 4px 8px;
    background: transparent;
    border: 1px solid transparent;
    cursor: pointer;
  }

  .name:hover {
    border-color: var(--border-subtle);
  }

  .said {
    font: var(--display-xs);
    letter-spacing: -0.01em;
    color: var(--text-primary);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .caret {
    flex: none;
    color: var(--text-muted);
  }

  .tools {
    margin-left: auto;
    display: flex;
    gap: 8px;
    --wails-draggable: no-drag;
  }

  .panels {
    flex: 1;
    min-height: 0;
    display: flex;
  }

  /* Bordered on both sides, so the pane header terminates against the rail and
     the docked pane instead of bleeding into them. */
  .agent {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
    border-left: 1px solid var(--border-subtle);
    border-right: 1px solid var(--border-subtle);
  }

  .assistant-mode {
    font: var(--machine-sm);
    font-size: 11px;
    color: var(--text-primary);
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
