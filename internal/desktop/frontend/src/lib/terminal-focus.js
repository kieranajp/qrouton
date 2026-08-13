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
