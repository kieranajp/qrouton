import { untrack } from "svelte";
import { screenFor } from "./deck.js";

/** Where a plan reader is standing and what moves them. The meter's phase is not
 * the screen a document opens on, so following moves the view only once it moves.
 * @param {{slides: () => any[], line: () => number, followed: () => number,
 * live: () => boolean, body: () => HTMLElement | undefined}} of */
export function deck({ slides, line, followed, live, body }) {
  let current = $state(untrack(() => screenFor(slides(), line())));
  let pinned = $state(false);
  // A mark answers the request that opened the pane; a remount must not revive
  // one the reader has already navigated away from.
  let retired = $state(false);
  let meter = untrack(() => followed());

  $effect(() => {
    const to = followed();
    untrack(() => {
      if (to === meter) return;
      meter = to;
      if (live() && !pinned) current = to;
    });
  });

  // A mark answers one open_file request, so navigating retires it. Every
  // control pins the reader; the bar's Follow button hands the position back.
  function show(screen, pin = true) {
    for (const marked of body()?.querySelectorAll(".marked") ?? [])
      marked.classList.remove("marked");
    retired = true;
    pinned = pin;
    current = Math.max(0, Math.min(screen, slides().length));
  }

  return {
    get current() {
      return current;
    },
    /** True while the pane is on the meter's phase and will stay with it. */
    get following() {
      return !pinned && followed() === current;
    },
    get retired() {
      return retired;
    },
    show,
    /** The reader's grip on the meter: taking it up moves them to the phase the
     * meter is on, letting it go leaves them where they are. */
    track(on) {
      if (on) show(followed(), false);
      else pinned = true;
    },
    /** Puts the pane back on the screen a fresh request asked for. */
    reload() {
      current = screenFor(slides(), line());
      pinned = false;
      retired = false;
    },
    /** Keeps the screen inside a plan that has since lost phases. */
    clamp(count) {
      if (current > count) current = count;
    },
  };
}
