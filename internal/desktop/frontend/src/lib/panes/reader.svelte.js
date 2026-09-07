import { untrack } from "svelte";

/** Which of a pane's two views is showing. A push carries the span along with
 * the text, so only a reload — which moves the epoch — reloads the pane.
 * @param {{structured: string, doc: () => {viewportEpoch?: number}, reload: () => void}} of */
export function reader({ structured, doc, reload }) {
  let mode = $state(structured);
  let epoch = untrack(() => doc().viewportEpoch);

  $effect(() => {
    const at = doc().viewportEpoch;
    untrack(() => {
      if (at === epoch) return;
      epoch = at;
      reload();
    });
  });

  return {
    get mode() {
      return mode;
    },
    set mode(next) {
      mode = next;
    },
    get reading() {
      return mode === "document";
    },
  };
}

/** Reports whichever element is scrolling, and null for a document with no
 * structure, which scrolls where every other pane does.
 * @param {{reading: () => boolean, structured: () => HTMLElement | undefined,
 * document: () => HTMLElement | undefined, when: () => boolean, onScroller: () => any}} of */
export function scrolls({ reading, structured, document, when, onScroller }) {
  $effect(() => {
    const scroller = reading() ? document() : structured();
    const tell = onScroller();
    tell?.(when() ? (scroller ?? null) : null);
    return () => tell?.(null);
  });
}
