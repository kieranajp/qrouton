/** terminalRenderer holds a WebGL context only while its terminal is shown. After a
 * loss it stays on DOM until hidden and shown again: a new context would evict another.
 * @param {{term: {loadAddon: (addon: any) => void}, enabled: boolean, create: () => any,
 *   watch: (addon: any) => (reason: string) => void}} options */
export function terminalRenderer({ term, enabled, create, watch }) {
  let addon;
  let unwatch;
  let fellBack = false;

  const drop = (reason) => {
    if (!addon) return;
    const current = addon;
    addon = undefined;
    current.dispose();
    unwatch(reason);
  };

  return {
    show() {
      if (!enabled || addon || fellBack) return;
      let next;
      try {
        next = create();
        term.loadAddon(next);
        const stop = watch(next);
        next.onContextLoss(() => {
          fellBack = true;
          drop("fallback");
        });
        addon = next;
        unwatch = stop;
      } catch (e) {
        next?.dispose();
        console.warn("webgl", e);
      }
    },
    hide() {
      fellBack = false;
      drop("released");
    },
    dispose() {
      drop("disposed");
    },
  };
}
