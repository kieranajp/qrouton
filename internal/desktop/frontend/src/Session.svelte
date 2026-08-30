<script>
  import Button from "./lib/core/Button.svelte";
  import CapsLabel from "./lib/core/CapsLabel.svelte";
  import Chip from "./lib/core/Chip.svelte";
  import CubeMark from "./lib/core/CubeMark.svelte";
  import Rail from "./lib/session/Rail.svelte";
  import Terminal from "./lib/session/Terminal.svelte";
  import ContextMenu from "./lib/shell/ContextMenu.svelte";
  import LatestDocument from "./lib/shell/LatestDocument.svelte";
  import Menu from "./lib/shell/Menu.svelte";
  import PaneHeader from "./lib/shell/PaneHeader.svelte";
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
    <CubeMark size={20} />
    <span class="name">{fields.identity}</span>
    {#if fields.branch}<span class="branch">{fields.branch}</span>{/if}
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
        <CapsLabel>Agent</CapsLabel>
        <Chip tone={fields.mode === "RPI" ? "guided" : "assistant"} selected>{fields.mode}</Chip>
        {#if fields.phase}<Chip>{fields.phase}</Chip>{/if}
        <LatestDocument
          latest={view.latest}
          count={fields.documents.length}
          open={view.listing}
          onToggle={view.hasDocuments ? () => (view.listing = !view.listing) : undefined}>
          <Menu
            label="Written this session"
            items={view.documentMenu}
            align="right"
            width={320}
            onSelect={(item) => view.read(item.path)} />
        </LatestDocument>
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
      <PickerOverlay slug={fields.slug} onClose={() => (view.added = "")} />
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

  /* A zero-size point for the menu to resolve its own position against. */
  .human {
    width: var(--w-human-pane);
    flex: none;
    display: flex;
    flex-direction: column;
    background: var(--surface-terminal);
  }
</style>
