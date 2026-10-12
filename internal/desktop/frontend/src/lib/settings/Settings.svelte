<script>
  import { tick } from "svelte";
  import Icon from "../core/Icon.svelte";
  import AdvancedTab from "./AdvancedTab.svelte";
  import FoldersTab from "./FoldersTab.svelte";
  import GeneralTab from "./GeneralTab.svelte";
  import GitHubTab from "./GitHubTab.svelte";
  import SearchTab from "./SearchTab.svelte";
  import StickersTab from "./StickersTab.svelte";
  import { TABS, erroredTabs, stepTab } from "./tabs.js";

  /** @type {{panel: ReturnType<typeof import("./settings.svelte.js").settings>}} */
  let { panel } = $props();

  let errored = $derived(erroredTabs(panel.fields));
  /** @type {Record<string, HTMLButtonElement>} */
  const buttons = {};

  /** @param {KeyboardEvent} event */
  async function key(event) {
    const next = stepTab(panel.tab, event.key);
    if (!next) return;
    event.preventDefault();
    event.stopPropagation();
    panel.tab = next;
    await tick();
    buttons[next]?.focus();
  }
</script>

<div class="settings">
  <nav class="index">
    <h1>Settings</h1>
    <div class="tabs" role="tablist" aria-orientation="vertical" tabindex="-1" onkeydown={key}>
      {#each TABS as item (item.key)}
        <button
          type="button"
          role="tab"
          id="settings-tab-{item.key}"
          class="tab"
          class:on={panel.tab === item.key}
          aria-selected={panel.tab === item.key}
          aria-controls="settings-panel"
          tabindex={panel.tab === item.key ? 0 : -1}
          bind:this={buttons[item.key]}
          onclick={() => (panel.tab = item.key)}>
          <span class="label">{item.label}</span>
          {#if errored.has(item.key)}
            <span class="marker failed" role="img" aria-label="has an error"></span>
          {:else if item.restart}
            <span class="marker restart" title="Applies after qrouton restarts"><Icon name="refresh" /></span>
          {/if}
        </button>
      {/each}
    </div>
    <span class="written">Written to config.json</span>
  </nav>

  <div class="panel" id="settings-panel" role="tabpanel" aria-labelledby="settings-tab-{panel.tab}">
    {#if panel.tab === "general"}
      <GeneralTab {panel} />
    {:else if panel.tab === "github"}
      <GitHubTab {panel} />
    {:else if panel.tab === "folders"}
      <FoldersTab {panel} />
    {:else if panel.tab === "search"}
      <SearchTab />
    {:else if panel.tab === "stickers"}
      <StickersTab {panel} />
    {:else}
      <AdvancedTab {panel} />
    {/if}
  </div>
</div>

<style>
  .settings {
    flex: 1;
    min-height: 0;
    display: flex;
  }

  .index {
    width: calc(220px * var(--ui-scale));
    flex: none;
    display: flex;
    flex-direction: column;
    gap: calc(2px * var(--ui-scale));
    padding: calc(22px * var(--ui-scale)) 0 calc(22px * var(--ui-scale)) calc(20px * var(--ui-scale));
    border-right: 1px solid var(--border-subtle);
  }

  h1 {
    margin: 0;
    padding: 0 calc(14px * var(--ui-scale)) calc(18px * var(--ui-scale));
    font: var(--display-md);
    letter-spacing: -0.03em;
    color: var(--text-primary);
  }

  .tabs {
    display: flex;
    flex-direction: column;
    gap: calc(2px * var(--ui-scale));
    outline: none;
  }

  .tab {
    display: flex;
    align-items: center;
    gap: calc(8px * var(--ui-scale));
    border: 0;
    border-radius: 0;
    background: transparent;
    padding: calc(9px * var(--ui-scale)) calc(14px * var(--ui-scale));
    font: var(--machine-sm);
    color: var(--text-muted);
    text-align: left;
    cursor: pointer;
  }

  .tab:hover {
    background: var(--wash-selected);
  }

  .tab.on {
    background: var(--surface-raised);
    box-shadow: var(--shadow-focus);
    color: var(--text-primary);
    font-weight: 700;
  }

  .tab:focus-visible {
    outline: 1px solid var(--accent-action);
    outline-offset: -1px;
  }

  .marker {
    margin-left: auto;
    display: flex;
    align-items: center;
  }

  .marker.failed {
    width: calc(7px * var(--ui-scale));
    height: calc(7px * var(--ui-scale));
    background: var(--state-failed);
  }

  .marker.restart {
    color: var(--text-faint);
  }

  .tab.on .marker.restart {
    color: var(--state-waiting);
  }

  .written {
    margin-top: auto;
    padding: 0 calc(14px * var(--ui-scale));
    font: var(--machine-xs);
    color: var(--text-faint);
  }

  .panel {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: calc(22px * var(--ui-scale));
    padding: calc(30px * var(--ui-scale)) calc(34px * var(--ui-scale));
    overflow: hidden auto;
  }
</style>
