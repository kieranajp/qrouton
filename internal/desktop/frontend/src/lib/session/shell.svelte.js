import { untrack } from "svelte";
import { escalate as escalateSession, pending as pendingAssembly } from "../assembly/calls.js";
import { assemblyOpen, pickerOpen } from "../assembly/steps.js";
import { chrome } from "../chrome.svelte.js";
import {
  closeWindow,
  openDocument,
  openShell,
  reorderWindow,
  selectWindow,
  surfaces,
} from "../docked.svelte.js";
import {
  humanWidth,
  readStored,
  roomFor,
  selectedTab,
  sidebarWidth,
  sidebarWidthKey,
  storedSidebarWidth,
  storedWidth,
  widthKey,
  writeStored,
} from "../layout.js";
import { relative } from "../relative.js";
import { revealPath } from "../sessions.js";
import { opensSettings } from "../shortcuts.js";
import {
  consumeTerminalFocus,
  focusGenerationIn,
  focusPendingIn,
  focusTerminal,
} from "../terminal-focus.js";
import { copyText, Events } from "../wails.js";

/**
 * shell is the session screen: the panes it splits the window into, the tabs Go
 * says are open, the documents this session has written, and whichever overlay
 * is covering the conversation.
 */
export function shell() {
  const session = chrome();
  let fields = $derived(session.fields);
  const open = surfaces(() => fields.slug);

  let dragged = $state(/** @type {Record<string, number>} */ ({}));
  let sidebarDragged = $state(storedSidebarWidth(readStored));
  let panels = $state(0);
  let railMeasured = $state(0);
  let measured = $state(0);

  let width = $derived(dragged[fields.slug] ?? storedWidth(readStored, fields.slug));
  let sidebarSize = $derived(sidebarWidth(sidebarDragged));
  let rail = $derived(sidebarSize || railMeasured);
  let room = $derived(roomFor(panels, rail));
  let human = $derived(humanWidth(width, room));

  const resize = (next) => (dragged = { ...dragged, [fields.slug]: next });

  function commit(next) {
    resize(next);
    writeStored(widthKey(fields.slug), next);
  }

  function commitSidebar(next) {
    sidebarDragged = next;
    writeStored(sidebarWidthKey(), next);
  }

  /** @type {Record<string, {generation: number, pending: boolean}>} */
  let terminalFocus = $state({});
  const request = (id) => {
    if (id) terminalFocus = focusTerminal(terminalFocus, id);
  };

  let selected = $derived(selectedTab(open.tabs, open.selected));

  // Go owns the selection, so a rejected Select leaves the strip where Go last
  // put it and the keyboard where it was.
  async function select(tab) {
    if (!tab?.id) return;
    try {
      await selectWindow(fields.slug, tab.id);
    } catch {
      return;
    }
    if (tab.kind === "terminal") request(tab.id);
  }

  // Go owns the order too, so a refused move leaves the strip as Go last drew it.
  async function reorder(from, to) {
    const tab = open.tabs[from];
    if (!tab?.id) return;
    try {
      await reorderWindow(fields.slug, tab.id, to);
    } catch {}
  }

  async function newShell() {
    try {
      await select({ id: await openShell(), kind: "terminal" });
    } catch {}
  }

  let listing = $state(false);

  // A session with no editor has nothing to open and no room here to say so.
  async function read(path) {
    listing = false;
    try {
      await openDocument(path);
    } catch {}
  }

  // The session on screen may have no rail row yet: a session is named by
  // adopting it, which happens once the window is already up.
  let conversations = $derived([
    ...(fields.terminal ? [{ terminal: fields.terminal, slug: fields.slug }] : []),
    ...fields.sessions.filter((row) => row.terminal && row.terminal !== fields.terminal),
  ]);

  let repositoryDocuments = $derived(
    fields.repositoryDocuments.map((repo) => ({
      label: repo.name,
      items: repo.documents.map((doc) => ({
        tag: doc.kind,
        id: doc.id,
        label: doc.name,
        path: doc.path,
      })),
    })),
  );

  let documentMenu = $derived([
    ...fields.documents.map((doc) => ({
      tag: doc.kind,
      id: doc.id,
      label: doc.name,
      meta: relative(doc.at, "compact"),
      path: doc.path,
    })),
    ...(repositoryDocuments.length ? ["-", { heading: "In-repo" }, ...repositoryDocuments] : []),
  ]);
  let hasDocuments = $derived(fields.documents.length > 0 || repositoryDocuments.length > 0);

  // The branch was reference material sitting in the titlebar as a label. It is
  // now behind the session's name, where the things you do with it also live.
  let identityOpen = $state(false);
  let editing = $derived(fields.repos.filter((repo) => repo.role === "editing" && repo.path));
  let identityMenu = $derived([
    ...(fields.branch ? [{ heading: "Branch" }, { label: fields.branch, disabled: true }, "-"] : []),
    { label: "Copy branch name", act: () => copyText(fields.branch), enabled: Boolean(fields.branch) },
    ...worktreeItems(),
  ]);

  // One editing repository is one worktree and one item. Several are several, and
  // naming them beats picking one and calling it the worktree.
  function worktreeItems() {
    if (!editing.length) return [];
    if (editing.length === 1) {
      const path = editing[0].path;
      return [
        { label: "Copy path to worktree", act: () => copyText(path) },
        { label: "Open worktree in Finder", act: () => revealWorktree(path) },
      ];
    }
    return [
      {
        label: "Copy path to worktree",
        items: editing.map((repo) => ({ label: repo.name, act: () => copyText(repo.path) })),
      },
      {
        label: "Open worktree in Finder",
        items: editing.map((repo) => ({ label: repo.name, act: () => revealWorktree(repo.path) })),
      },
    ];
  }

  async function revealWorktree(path) {
    identityOpen = false;
    try {
      await revealPath(fields.slug, path);
    } catch {}
  }

  function chose(item) {
    identityOpen = false;
    item?.act?.();
  }

  let requested = $state(false);
  let escalating = $state(false);
  let settingsOpen = $state(false);
  // An escalation is the shown session's own pending request, so switching
  // session takes its picker with it. Add-repos is this page's, and belongs to
  // the session it was pressed on for the same reason.
  let added = $state("");
  let assembling = $derived(assemblyOpen(requested, session.settled, fields.slug));
  let picker = $derived(pickerOpen(fields.slug, fields.picker, added));
  let covered = $derived(fields.welcoming || assembling || picker || settingsOpen);

  async function escalate() {
    if (escalating || fields.mode !== "ASSISTANT") return;
    escalating = true;
    try {
      await escalateSession(fields.slug);
    } catch {}
    escalating = false;
  }

  $effect(() => {
    let live = true;
    const off = Events.On("assembly:requested", () => (requested = true));
    pendingAssembly()
      .then((ticket) => {
        if (live && ticket) requested = true;
      })
      .catch(() => {});
    const onKey = (event) => {
      if (!opensSettings(event)) return;
      event.preventDefault();
      if (!fields.welcoming) settingsOpen = true;
    };
    window.addEventListener("keydown", onKey);
    return () => {
      live = false;
      off();
      window.removeEventListener("keydown", onKey);
    };
  });

  // Arriving at a session is a surface the user asked for, so its conversation
  // takes the keyboard; every other chrome payload leaves it alone.
  let shown = "";
  $effect(() => {
    const id = fields.terminal;
    if (id === shown) return;
    shown = id;
    untrack(() => request(id));
  });

  // An overlay covering the terminal hands the keyboard back when it goes, the
  // same way the rail's menu and its confirm do. Watching the state rather than
  // the dismissal catches the picker the backend closes, which no handler here
  // ever sees.
  let wasCovered = false;
  $effect(() => {
    const back = wasCovered && !covered;
    wasCovered = covered;
    if (back) untrack(() => request(fields.terminal));
  });

  return {
    get fields() {
      return fields;
    },
    get panels() {
      return panels;
    },
    set panels(value) {
      panels = value;
    },
    get railMeasured() {
      return railMeasured;
    },
    set railMeasured(value) {
      railMeasured = value;
    },
    get measured() {
      return measured;
    },
    set measured(value) {
      measured = value;
    },
    get sidebarSize() {
      return sidebarSize;
    },
    get rail() {
      return rail;
    },
    get room() {
      return room;
    },
    get human() {
      return human;
    },
    resize,
    commit,
    reset: () => commit(0),
    resizeSidebar: (next) => (sidebarDragged = next),
    commitSidebar,
    resetSidebar: () => commitSidebar(0),

    get tabs() {
      return open.tabs;
    },
    get selected() {
      return selected;
    },
    select,
    reorder,
    close: (tab) => tab && closeWindow(tab.id),
    newShell,

    get conversations() {
      return conversations;
    },
    focusOf: (id) => focusGenerationIn(terminalFocus, id),
    focusPendingOf: (id) => focusPendingIn(terminalFocus, id),
    focused: (id, generation) =>
      (terminalFocus = consumeTerminalFocus(terminalFocus, id, generation)),
    handBack: () => request(fields.terminal),

    get identityOpen() {
      return identityOpen;
    },
    set identityOpen(value) {
      identityOpen = value;
    },
    get identityMenu() {
      return identityMenu;
    },
    chose,
    get documentMenu() {
      return documentMenu;
    },
    get hasDocuments() {
      return hasDocuments;
    },
    get listing() {
      return listing;
    },
    set listing(value) {
      listing = value;
    },
    read,

    get escalating() {
      return escalating;
    },
    escalate,

    get assembling() {
      return assembling;
    },
    get picker() {
      return picker;
    },
    get settingsOpen() {
      return settingsOpen;
    },
    set settingsOpen(value) {
      settingsOpen = value;
    },
    get requested() {
      return requested;
    },
    set requested(value) {
      requested = value;
    },
    get added() {
      return added;
    },
    set added(value) {
      added = value;
    },
  };
}
