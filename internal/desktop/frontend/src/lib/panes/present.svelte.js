/** The presentation on screen, if any. One full-viewport surface for the whole
 * window, so the index lives in the module rather than in a pane that stops
 * being drawn the moment another tab is selected. */
export const present = presenting();

const clamp = (at, total) => Math.max(0, Math.min(at, total - 1));

function presenting() {
  let owner = $state("");
  let index = $state(0);
  let source = () => /** @type {{html: string, title: string, notes: string}[]} */ ([]);

  return {
    get active() {
      return owner !== "";
    },
    get cards() {
      return source();
    },
    get current() {
      return index;
    },
    get total() {
      return source().length;
    },
    /** @param {string} by @param {() => {html: string, title: string, notes: string}[]} cards @param {number} [from] */
    open(by, cards, from = 0) {
      owner = by;
      source = cards;
      index = clamp(from, cards().length);
    },
    /** A pane ends only its own presentation, so a deck deactivating in the
     * background cannot close someone else's.
     * @param {string} by */
    close(by) {
      if (!owner || by !== owner) return;
      this.leave();
    },
    leave() {
      owner = "";
      source = () => [];
      index = 0;
    },
    /** @param {number} by */
    step(by) {
      index = clamp(index + by, source().length);
    },
    /** @param {number} to */
    go(to) {
      index = clamp(to, source().length);
    },
  };
}
