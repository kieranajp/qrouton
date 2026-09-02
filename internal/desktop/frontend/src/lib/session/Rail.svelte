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
  import { cleanup, cycleSticker, reload, reveal, show, uncommitted } from "../sessions.js";
  import { age } from "../relative.js";
  import { rowAt, shortcut } from "../shortcuts.js";
  import { DEFAULT_STICKER_LABELS, stickerFeedback } from "./stickers.js";

  /**
   * @type {{
   *   sessions: any[],
   *   slug: string,
   *   repos: any[],
   *   agents: any,
   *   stickerLabels: Record<string, string>,
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
    stickerLabels = DEFAULT_STICKER_LABELS,
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
    { label: "Reload", act: reloadRow },
    { label: "Reveal in Finder", act: revealRow },
    "-",
    { label: "Clean up…", tone: "destructive", act: askCleanup },
  ];

  const stickerQueues = new Map();
  let pendingStickers = $state({});
  /** @type {{slug: string, sequence: number, text: string, failed: boolean} | null} */
  let rowNotice = $state(null);
  let noticeSequence = 0;
  let rowNoticeTimer;
  let mounted = true;

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
    return () => {
      mounted = false;
      window.removeEventListener("keydown", onKey);
      clearTimeout(rowNoticeTimer);
    };
  });

  function showRowNotice(slug, text, failed = false) {
    if (!mounted) return;
    clearTimeout(rowNoticeTimer);
    rowNotice = { slug, text, failed, sequence: ++noticeSequence };
    rowNoticeTimer = setTimeout(() => {
      rowNotice = null;
    }, 1500);
  }

  function changeSticker(event, row) {
    event.stopPropagation();
    const slug = row.slug;
    pendingStickers = { ...pendingStickers, [slug]: (pendingStickers[slug] ?? 0) + 1 };

    const previous = stickerQueues.get(slug) ?? Promise.resolve();
    const queued = previous
      .catch(() => {})
      .then(() => cycleSticker(slug))
      .then((committed) => showRowNotice(slug, stickerFeedback(committed, stickerLabels)))
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
  <!-- A band the height of a pane header, so the first session row starts on the
       same line as the transcript and the tab strip beside it. No bottom border:
       the alignment is the point, and a rule would make it read as a header cell. -->
  <div class="band">
    <CapsLabel tone="dim" centred>Sessions</CapsLabel>
  </div>

  <div class="session-list" aria-label="Sessions">
    {#each sessions as row, i (row.slug)}
      <RailItem
        initials={row.initials}
        shortcut={shortcut(i)}
        name={row.name}
        repos={row.repos}
        summary={row.summary}
        idle={row.summary?.running ? "" : age(row.opened)}
        selected={row.slug === slug}
        unseen={row.unseen}
        stickerId={row.sticker}
        {stickerLabels}
        stickerBusy={(pendingStickers[row.slug] ?? 0) > 0}
        feedback={rowNotice?.slug === row.slug ? rowNotice : null}
        onSelect={() => show(row.slug)}
        onSticker={(event) => changeSticker(event, row)}
        onContextMenu={(event) => openMenu(event, row)} />
    {/each}

    <Button variant="cube" size="sm" glyph="+" wide onclick={onNewSession}>New session</Button>
    <span class="allowance"></span>
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
    background: var(--surface-chrome);
    overflow: hidden;
  }

  /* Centred in the band, not sat on its floor: AGENT is centred in the pane
     header beside it, so this is where the two headings share a line. */
  .band {
    height: var(--h-pane-header);
    box-sizing: border-box;
    flex: none;
    display: flex;
    align-items: center;
    justify-content: center;
    padding: 0 12px;
  }

  /* Sessions is the only greedy child. Activity and this-session size to what
     they hold, so an empty panel occupies nothing rather than a share of the
     rail. */
  .session-list {
    flex: 1 1 auto;
    min-height: 0;
    overflow: hidden auto;
    display: flex;
    flex-direction: column;
    gap: 4px;
    padding: 0 12px;
  }

  .allowance {
    flex: none;
    height: 12px;
  }

  .detail-stack {
    flex: 0 1 auto;
    min-height: 0;
    display: flex;
    flex-direction: column;
    gap: 10px;
    padding: 0 12px 12px;
  }

  /* Live work outranks the repository list, which is reference: activity takes
     what it needs up to its cap, and the list below scrolls for the rest. */
  .activity-scroll {
    flex: 0 0 auto;
    min-height: 0;
    max-height: 250px;
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
