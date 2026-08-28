<script>
  import { onMount } from "svelte";
  import Button from "../core/Button.svelte";
  import CapsLabel from "../core/CapsLabel.svelte";
  import { dismissible } from "../core/dismiss.js";
  import Confirm from "../shell/Confirm.svelte";
  import Menu from "../shell/Menu.svelte";
  import ActivityPanel from "./ActivityPanel.svelte";
  import RailItem from "./RailItem.svelte";
  import RepoList from "./RepoList.svelte";
  import { menuHeight, place } from "../menu.js";
  import { cleanup, reveal, show, uncommitted } from "../sessions.js";
  import { rowAt, shortcut } from "../shortcuts.js";

  /**
   * @type {{
   *   sessions: any[],
   *   slug: string,
   *   repos: any[],
   *   agents: any,
   *   onNewSession: () => void,
   *   onAddRepos: () => void,
   *   onDismissed: () => void,
   *   size: number,
   *   width: number,
   * }}
   */
  let {
    sessions,
    slug,
    repos,
    agents,
    onNewSession,
    onAddRepos,
    onDismissed,
    size = 0,
    // The splitter needs to know how much room the rail leaves the panes, and
    // only the rail knows how wide it drew itself.
    width = $bindable(0),
  } = $props();

  const ROW_MENU_WIDTH = 190;
  const ROW_MENU = [
    { label: "Reveal in Finder", act: revealRow },
    "-",
    { label: "Clean up…", tone: "destructive", act: askCleanup },
  ];

  // A rail row's number is its position, so it moves when showing a session
  // re-sorts the list.
  onMount(() => {
    const onKey = (event) => {
      const row = rowAt(sessions, event);
      if (!row) return;
      event.preventDefault();
      show(row.slug);
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  });

  /** @type {{row: any, x: number, y: number} | null} */
  let menu = $state(null);
  /** @type {{row: any, dirty: string[], checked: boolean, error: string} | null} */
  let confirming = $state(null);
  let cleaning = $state(false);

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
    onDismissed();
  }

  // Nothing here changed, and the rail has no surface to report a refusal on.
  async function revealRow(row) {
    onDismissed();
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
    onDismissed();
  }
</script>

<div class="rail" style:width={size ? size + "px" : null} bind:clientWidth={width}>
  <div class="session-list" aria-label="Sessions">
    <CapsLabel>Sessions</CapsLabel>
    {#each sessions as row, i (row.slug)}
      <RailItem
        initials={row.initials}
        shortcut={shortcut(i)}
        name={row.name}
        repos={row.repos}
        summary={row.summary}
        selected={row.slug === slug}
        unseen={row.unseen}
        onclick={() => show(row.slug)}
        oncontextmenu={(event) => openMenu(event, row)} />
    {/each}

    <Button variant="dashed" size="sm" glyph="+" onclick={onNewSession}>New session</Button>
  </div>

  <div class="detail-stack" aria-label="Selected session details">
    <div class="activity-scroll">
      <ActivityPanel {agents} />
    </div>
    <RepoList {repos} {onAddRepos} />
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

<style>
  .rail {
    width: var(--w-rail);
    box-sizing: border-box;
    flex: none;
    display: flex;
    flex-direction: column;
    gap: 10px;
    padding: 14px 12px;
    background: var(--surface-chrome);
    overflow: hidden;
  }

  .session-list,
  .detail-stack {
    min-height: 0;
    overflow: hidden auto;
  }

  .session-list {
    flex: 1 1 50%;
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .detail-stack {
    flex: 1 1 50%;
    display: flex;
    flex-direction: column;
    gap: 10px;
  }

  .activity-scroll {
    flex: 0 0 60%;
    min-height: 0;
    overflow: hidden auto;
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
</style>
