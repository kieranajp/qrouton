import { Call, Events } from "./wails.js";

const WINDOWS_SERVICE = "github.com/kieranajp/qrouton/internal/desktop.Windows";

const NONE = { tabs: [], floating: [] };

/** surfaces is where each window the workbench has open is drawn. */
export function surfaces() {
  let open = $state(NONE);
  let live = false;
  const apply = (value) => (open = { ...NONE, ...(value || {}) });
  Events.On("window:open", (event) => {
    live = true;
    apply(event.data);
  });
  // The initial pull can resolve after an event has already landed; a stale
  // snapshot must not overwrite it.
  Call.ByName(WINDOWS_SERVICE + ".Surfaces").then((value) => {
    if (!live) apply(value);
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
