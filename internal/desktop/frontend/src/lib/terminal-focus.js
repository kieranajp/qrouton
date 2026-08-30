export function createTerminalActivation({ frame, cancelFrame, refit, focus, handled }) {
  let active = false;
  let generation = 0;
  let focusFrame;
  let refitFrame;

  const scheduleRefit = () => {
    if (refitFrame !== undefined) return;
    refitFrame = frame(() => {
      refitFrame = undefined;
      if (active) refit();
    });
  };

  const scheduleFocus = () => {
    if (focusFrame !== undefined) return;
    focusFrame = frame(() => {
      focusFrame = undefined;
      if (!active) return;
      focus();
      handled(generation);
    });
  };

  return {
    update(nextActive, nextGeneration, focusPending) {
      if (nextActive && !active) scheduleRefit();
      active = nextActive;
      generation = nextGeneration;
      if (active && focusPending) scheduleFocus();
    },
    destroy() {
      active = false;
      if (refitFrame !== undefined) cancelFrame(refitFrame);
      if (focusFrame !== undefined) cancelFrame(focusFrame);
      refitFrame = undefined;
      focusFrame = undefined;
    },
  };
}

// The page's half of the same handshake: a request carries a generation the
// terminal acknowledges once it has taken the keyboard, so a focus asked for
// before the terminal mounts is delivered exactly once.

export const focusGenerationIn = (generations, id) => generations[id]?.generation ?? 0;

export const focusPendingIn = (generations, id) => generations[id]?.pending ?? false;

export const focusTerminal = (generations, id) => ({
  ...generations,
  [id]: { generation: focusGenerationIn(generations, id) + 1, pending: true },
});

export function consumeTerminalFocus(generations, id, generation) {
  const current = generations[id];
  if (!current || current.generation !== generation || !current.pending) return generations;
  return { ...generations, [id]: { generation, pending: false } };
}
