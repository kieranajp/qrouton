import { Call, Events } from "./wails.js";

const WINDOWS_SERVICE = "github.com/kieranajp/qrouton/internal/desktop.Windows";

const NONE = { tabs: [], floating: [] };

/**
 * surfaces is where each window one session has open is drawn. Another
 * session's windows are ignored: the event reaches every page in the process.
 * @param {() => string} slug
 */
export function surfaces(slug) {
  let open = $state(NONE);
  let live = false;
  const apply = (value) => (open = { ...NONE, ...(value || {}) });
  Events.On("window:open", (event) => {
    if (event.data?.session !== slug()) return;
    live = true;
    apply(event.data);
  });
  // A session's windows are open before its page subscribes, so each is pulled
  // once — and a pull resolving after an event must not overwrite it.
  let pulled;
  $effect(() => {
    const session = slug();
    if (session === pulled) return;
    pulled = session;
    live = false;
    apply(NONE);
    Call.ByName(WINDOWS_SERVICE + ".Surfaces", session).then((value) => {
      if (!live && pulled === session) apply(value);
    });
  });
  return {
    get tabs() {
      return open.tabs;
    },
    get floating() {
      return open.floating;
    },
  };
}

export const closeWindow = (id) => Call.ByName(WINDOWS_SERVICE + ".Close", id);
export const openShell = () => Call.ByName(WINDOWS_SERVICE + ".OpenShell");

/** openDocument opens a session document, or selects the tab already on it. */
export const openDocument = (path) => Call.ByName(WINDOWS_SERVICE + ".OpenDocument", path);

/**
 * whenSelected runs on a window the Go side wants shown — a document the agent
 * opened, which would otherwise render behind whatever tab is up.
 * @param {() => string} slug
 * @param {(id: string) => void} select
 */
export const whenSelected = (slug, select) =>
  Events.On("window:select", (event) => {
    if (event.data?.session === slug()) select(event.data.id);
  });
