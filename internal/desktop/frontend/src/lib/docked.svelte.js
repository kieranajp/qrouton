import {
  WINDOWS_CLOSE,
  WINDOWS_EVENT,
  WINDOWS_OPEN_DOCUMENT,
  WINDOWS_OPEN_SHELL,
  WINDOWS_REORDER,
  WINDOWS_SELECT,
  WINDOWS_SURFACES,
} from "./bridge/generated.js";
import { Call, call, follow } from "./wails.js";

/** @typedef {import("./bridge/generated.js").WindowIcon} WindowIcon */
/** @typedef {import("./bridge/generated.js").TabStatus} TabStatus */
/** @typedef {{id: string, label: string, badge?: string, icon?: WindowIcon, kind: string, status?: TabStatus, pinned?: boolean, artifact?: string}} DockedTab */

/** @type {{tabs: DockedTab[], selected: string}} */
const NONE = { tabs: [], selected: "" };

/** @param {() => string} slug */
export function surfaces(slug) {
  let open = $state(NONE);
  const apply = (value) => (open = { ...NONE, ...(value || {}) });
  // Compared by value, so a chrome update that keeps the slug keeps the subscription.
  const current = $derived(slug());
  $effect(() => {
    const session = current;
    apply(NONE);
    return follow(WINDOWS_EVENT, () => call(Call.ByName(WINDOWS_SURFACES, session)), apply, {
      accepts: (data) => data?.session === session,
    }).stop;
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

export const closeWindow = (id) => Call.ByName(WINDOWS_CLOSE, id);
export const openShell = () => Call.ByName(WINDOWS_OPEN_SHELL);

export const selectWindow = (slug, id) => Call.ByName(WINDOWS_SELECT, slug, id);

/** reorderWindow moves a tab to an index in its session's whole tab strip. */
export const reorderWindow = (slug, id, to) => Call.ByName(WINDOWS_REORDER, slug, id, to);

/** openDocument opens a session document, or selects the tab already on it. */
export const openDocument = (path) => Call.ByName(WINDOWS_OPEN_DOCUMENT, path);
