let nextRequest = 0;

/** @param {(request: number) => Promise<any[]>} render
 * @param {(results: any[]) => void} apply */
export function createDiagramStream(render, apply) {
  let current = 0;
  const receive = (results) => {
    if (!current) return;
    apply((results ?? []).filter((result) => result.request === current));
  };
  return {
    draw: () => {
      current = ++nextRequest;
      return render(current).then(receive).catch(() => {});
    },
    receive,
    destroy: () => { current = 0; },
  };
}
