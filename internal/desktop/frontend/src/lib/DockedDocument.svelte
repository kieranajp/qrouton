<script>
  import { onDestroy, onMount, tick } from "svelte";
  import { chrome } from "./chrome.svelte.js";
  import DocumentPane from "./DocumentPane.svelte";
  import FindBar from "./FindBar.svelte";
  import { activateMatch, clearMatches, findShortcut, markMatches } from "./find.js";
  import TerminalPane from "./shell/TerminalPane.svelte";

  /** @type {{id: string, active?: boolean}} */
  let { id, active = false } = $props();

  const session = chrome();

  /** @type {HTMLElement} */
  let body = $state();
  /** A pane with a footer scrolls inside itself; every other pane scrolls here. */
  let owned = $state(/** @type {HTMLElement | null} */ (null));
  let scrollRoot = $derived(owned ?? body);
  /** @type {HTMLElement} */
  let content = $state();
  /** @type {HTMLInputElement} */
  let findField = $state();
  let finding = $state(false);
  let query = $state("");
  let count = $state(0);
  let current = $state(-1);
  /** @type {HTMLElement[][]} */
  let matches = [];

  function refresh(value) {
    query = value;
    matches = markMatches(content, query);
    count = matches.length;
    current = activateMatch(matches, 0);
  }

  function move(by) {
    current = activateMatch(matches, current + by);
  }

  async function openFind() {
    if (!finding) {
      finding = true;
      await tick();
      if (!active || !finding) return;
      refresh(query);
    }
    findField?.focus();
    findField?.select();
  }

  function closeFind() {
    clearMatches(content);
    matches = [];
    count = 0;
    current = -1;
    finding = false;
    if (active) scrollRoot?.focus({ preventScroll: true });
  }

  async function documentReady() {
    if (!finding) return;
    await tick();
    if (!active || !finding) return;
    refresh(query);
  }

  onMount(() => {
    const key = (event) => {
      if (!active || !findShortcut(event)) return;
      event.preventDefault();
      event.stopPropagation();
      openFind();
    };
    // Capture keeps Control-F from reaching the conversation terminal first;
    // when a shell tab is selected, no document is active and xterm keeps it.
    window.addEventListener("keydown", key, true);
    return () => window.removeEventListener("keydown", key, true);
  });

  $effect(() => {
    if (!active && finding) closeFind();
  });

  onDestroy(() => clearMatches(content));
</script>

<TerminalPane flush style="display: {active ? 'flex' : 'none'}">
  {#if finding}
    <FindBar
      {query}
      {count}
      {current}
      bind:field={findField}
      onQuery={refresh}
      onPrevious={() => move(-1)}
      onNext={() => move(1)}
      onClose={closeFind} />
  {/if}
  <div class="body" class:bound={owned} bind:this={body} tabindex="-1">
    <div bind:this={content}>
      <DocumentPane
        {id}
        {active}
        {scrollRoot}
        agentWorking={session.fields.activity === "working"}
        onReady={documentReady}
        onScroller={(element) => (owned = element)} />
    </div>
  </div>
</TerminalPane>

<style>
  .body {
    flex: 1;
    min-height: 0;
    overflow-y: auto;
    outline: none;
  }

  /* The find bar marks inside this wrapper, and a pane with chrome of its own
     needs the port's full height to place it against. */
  .body > :global(div) {
    display: flex;
    flex-direction: column;
    min-height: 100%;
  }

  /* A pane that scrolls inside itself needs a floor to shrink against, and this
     one must not scroll as well: two bars for one document. */
  .bound {
    overflow-y: hidden;
  }

  .bound > :global(div) {
    height: 100%;
    min-height: 0;
  }

  .body :global(mark[data-document-find]) {
    background: color-mix(in srgb, var(--accent-action) 28%, transparent);
    color: inherit;
  }

  .body :global(mark[data-document-find].current) {
    background: var(--accent-action);
    color: var(--text-on-accent);
  }
</style>
