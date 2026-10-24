import { onDestroy } from "svelte";

const FLASH_MS = 1500;

/** A value shown for a moment. Showing again restarts the moment.
 * Call it while a component initialises; the timer stops with the component.
 * @template T
 * @param {number} [ms] */
export function flash(ms = FLASH_MS) {
  /** @type {T | undefined} */
  let value = $state();
  /** @type {ReturnType<typeof setTimeout> | undefined} */
  let timer;
  onDestroy(() => clearTimeout(timer));
  return {
    get value() {
      return value;
    },
    /** @param {T} next */
    show(next) {
      clearTimeout(timer);
      value = next;
      timer = setTimeout(() => (value = undefined), ms);
    },
    clear() {
      clearTimeout(timer);
      value = undefined;
    },
  };
}
