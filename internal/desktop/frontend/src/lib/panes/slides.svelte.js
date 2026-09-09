/** @param {{cards: () => unknown[], body: () => HTMLElement | undefined}} of */
export function slides({ cards, body }) {
  let current = $state(0);
  const bounded = (index) => Math.max(0, Math.min(index, cards().length - 1));
  const elements = () => [...(body()?.querySelectorAll(":scope > .stack > .card") ?? [])];
  let measured;
  let found = null;

  $effect(() => {
    current = bounded(current);
  });

  return {
    get current() {
      return current;
    },
    show(index) {
      current = bounded(index);
      elements()[current]?.scrollIntoView({ block: "start" });
    },
    measure() {
      const root = body();
      const viewport = root?.getBoundingClientRect();
      if (!root || !viewport || viewport.height <= 0 || viewport.width <= 0) return;
      const rendered = elements();
      const match = root.querySelector("mark[data-document-find].current");
      const target = match !== found ? match : measured !== root ? root.querySelector(".marked") : null;
      found = match;
      measured = root;
      if (target) {
        const selected = rendered.findIndex((card) => card.contains(target));
        if (selected >= 0) {
          current = bounded(selected);
          const card = rendered[selected];
          const targetBottom = target.getBoundingClientRect().bottom - card.getBoundingClientRect().top;
          if (!match || targetBottom <= viewport.height) {
            card.scrollIntoView({ block: "start" });
          }
          return;
        }
      }
      if (
        root.scrollHeight > root.clientHeight &&
        root.scrollTop + root.clientHeight >= root.scrollHeight - 2
      ) {
        current = bounded(cards().length - 1);
        return;
      }
      const index = rendered.findIndex((card) => {
        const rect = card.getBoundingClientRect();
        return rect.bottom > viewport.top + 1 && rect.top < viewport.bottom;
      });
      if (index >= 0) current = bounded(index);
    },
  };
}
