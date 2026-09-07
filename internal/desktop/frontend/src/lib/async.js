// Scheduling for calls that cross the bridge, kept pure: node --test is the
// whole frontend harness.

/** Bridge calls may settle out of order; only the newest answer lands.
 * @template V, A
 * @param {number} ms @param {(value: V) => Promise<A>} ask resolves errors as values
 * @param {(answer: A) => void} land @param {{set?: (run: () => void, ms: number) => any, clear?: (timer: any) => void}} [timers] */
export function debounced(ms, ask, land, { set = setTimeout, clear = clearTimeout } = {}) {
  let timer;
  let asked = 0;

  /** @param {V} value */
  const schedule = (value) => {
    if (timer !== undefined) clear(timer);
    timer = set(async () => {
      timer = undefined;
      const mine = ++asked;
      const answer = await ask(value);
      if (mine === asked) land(answer);
    }, ms);
  };

  // A call already with the bridge cannot be recalled, so cancel drops its
  // answer as well as the value still waiting on the timer.
  const cancel = () => {
    asked++;
    if (timer === undefined) return;
    clear(timer);
    timer = undefined;
  };

  return { schedule, cancel };
}
