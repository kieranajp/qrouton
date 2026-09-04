/** Which card the reader is standing on, read off what the viewport last
 * measured. The pane shows it as a counter; a stepped mode would swap the
 * container around the same list without touching the render.
 * @param {{cards: () => {line: number, lineEnd: number}[]}} of */
export function slides({ cards }) {
  let current = $state(0);

  return {
    get current() {
      return current;
    },
    /** @param {{intervals: {line: number, to: number}[]}} state */
    measure(state) {
      const first = state?.intervals?.[0];
      if (!first) return;
      const at = cards().findIndex((card) => card.lineEnd >= first.line);
      if (at >= 0) current = at;
    },
  };
}
