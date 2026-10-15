import { tick, untrack } from "svelte";
import { SWITCHER_FAILED } from "../bridge/generated.js";
import { matchSessions, initialSelection, reconcileSelection } from "./switcher.js";

export function sessionSwitcher({ fields, blocked, covered, show, clearFocus, request }) {
  let opened = $state(false);
  let query = $state("");
  let active = $state("");
  let busy = $state(false);
  let error = $state("");
  let target = $state("");
  let succeeded = $state(false);
  let deferredError = false;
  let generation = 0;
  let origin = "";
  let originElement;
  let originTerminal = "";
  let matches = $derived(matchSessions(fields().sessions, query));
  let visible = $derived(opened && !covered());

  function reset() {
    opened = false;
    busy = false;
    target = "";
    succeeded = false;
    deferredError = false;
    generation++;
  }

  async function restore() {
    clearFocus();
    await tick();
    if (covered() || opened) return;
    if (fields().slug === origin && originTerminal && originElement?.isConnected && originElement.getClientRects().length) request(originTerminal);
    else if (fields().slug === origin && originElement?.isConnected && originElement.getClientRects().length) originElement.focus();
    else request(fields().terminal);
  }

  function cancel() {
    if (!opened || busy) return;
    reset();
    restore();
  }

  function open() {
    if (opened) { cancel(); return; }
    if (blocked() || target) return;
    origin = fields().slug;
    originElement = document.activeElement;
    originTerminal = originElement?.closest("[data-terminal-id]")?.getAttribute("data-terminal-id") ?? "";
    clearFocus();
    query = "";
    error = "";
    active = initialSelection(matchSessions(fields().sessions), origin);
    opened = true;
    generation++;
  }

  async function activate(slug = active) {
    if (!opened || busy || !matches.some(({ row }) => row.slug === slug)) return;
    active = slug;
    if (slug === fields().slug) { cancel(); return; }
    busy = true;
    error = "";
    target = slug;
    const attempt = ++generation;
    try {
      await show(slug);
      if (attempt !== generation) return;
      busy = false;
      opened = false;
      succeeded = true;
      clearFocus();
    } catch {
      if (attempt !== generation) return;
      busy = false;
      target = "";
      if (fields().slug === origin) {
        deferredError = covered();
        error = SWITCHER_FAILED;
      } else {
        reset();
        clearFocus();
        request(fields().terminal);
      }
    }
  }

  $effect(() => {
    const next = reconcileSelection(matches, active);
    if (next !== active) active = next;
  });
  $effect(() => {
    const slug = fields().slug;
    const terminal = fields().terminal;
    const overlay = covered();
    const searching = opened;
    const destination = target;
    const completed = succeeded;
    const pending = busy;
    const failedWhileCovered = error && deferredError;
    untrack(() => {
      if (!searching && !destination) return;
      if (slug !== origin && slug !== destination) { reset(); clearFocus(); return; }
      if (overlay && searching && !pending && !failedWhileCovered) { reset(); return; }
      if (!overlay) deferredError = false;
      if (completed && slug === destination && terminal && !overlay) {
        reset();
        clearFocus();
        request(terminal);
      }
    });
  });

  return {
    get visible() { return visible; },
    get ownsFocus() { return opened || Boolean(target); },
    get busy() { return busy; },
    get error() { return error; },
    get query() { return query; },
    set query(value) { if (!busy) { query = value; active = matchSessions(fields().sessions, query)[0]?.row.slug ?? ""; } },
    get active() { return active; },
    get matches() { return matches; },
    open, cancel, activate,
    move(delta) {
      if (busy || !matches.length) return;
      const index = matches.findIndex(({ row }) => row.slug === active);
      active = matches[(index + delta + matches.length) % matches.length].row.slug;
    },
  };
}
