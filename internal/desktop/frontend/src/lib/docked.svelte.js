import {
  WINDOWS_CLOSE,
  WINDOWS_EVENT,
  WINDOWS_OPEN_DOCUMENT,
  WINDOWS_OPEN_SHELL,
  WINDOWS_REORDER,
  WINDOWS_SELECT,
  WINDOWS_SURFACES,
} from "./bridge/generated.js";
import { Call, Events, call } from "./wails.js";

const NONE = { tabs: [], selected: "" };

/** @param {() => string} slug */
export function surfaces(slug) {
  let open = $state(NONE);
  let live = false;
  const apply = (value) => (open = { ...NONE, ...(value || {}) });
  Events.On(WINDOWS_EVENT, (event) => {
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
    call(Call.ByName(WINDOWS_SURFACES, session)).then((answer) => {
      if (answer.ok && !live && pulled === session) apply(answer.value);
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

export const closeWindow = (id) => Call.ByName(WINDOWS_CLOSE, id);
export const openShell = () => Call.ByName(WINDOWS_OPEN_SHELL);

export const selectWindow = (slug, id) => Call.ByName(WINDOWS_SELECT, slug, id);

/** reorderWindow moves a tab to an index in its session's whole tab strip. */
export const reorderWindow = (slug, id, to) => Call.ByName(WINDOWS_REORDER, slug, id, to);

/** openDocument opens a session document, or selects the tab already on it. */
export const openDocument = (path) => Call.ByName(WINDOWS_OPEN_DOCUMENT, path);
