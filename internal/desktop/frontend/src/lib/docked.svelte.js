import {
  CLOSE_WINDOW,
  OPEN_DOCUMENT,
  OPEN_SHELL,
  SELECT_WINDOW,
  SURFACES,
} from "./session/services.js";
import { Call, Events, call } from "./wails.js";

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
    call(Call.ByName(SURFACES, session)).then((answer) => {
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

export const closeWindow = (id) => Call.ByName(CLOSE_WINDOW, id);
export const openShell = () => Call.ByName(OPEN_SHELL);

export const selectWindow = (slug, id) => Call.ByName(SELECT_WINDOW, slug, id);

/** openDocument opens a session document, or selects the tab already on it. */
export const openDocument = (path) => Call.ByName(OPEN_DOCUMENT, path);
