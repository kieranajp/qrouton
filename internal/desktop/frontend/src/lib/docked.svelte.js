import { Call, Events } from "./wails.js";

const WINDOWS_SERVICE = "github.com/kieranajp/qrouton/internal/desktop.Windows";

const NONE = { tabs: [], selected: "" };

/** @param {() => string} slug */
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
    get selected() {
      return open.selected;
    },
  };
}

export const closeWindow = (id) => Call.ByName(WINDOWS_SERVICE + ".Close", id);
export const openShell = () => Call.ByName(WINDOWS_SERVICE + ".OpenShell");

export const selectWindow = (slug, id) =>
  Call.ByName(WINDOWS_SERVICE + ".Select", slug, id);

/** openDocument opens a session document, or selects the tab already on it. */
export const openDocument = (path) => Call.ByName(WINDOWS_SERVICE + ".OpenDocument", path);
