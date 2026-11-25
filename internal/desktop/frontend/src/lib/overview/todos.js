import { todoLabel } from "../session/rail.js";

/** @typedef {{path: string, title: string, open: number}} TodoDocument */

/** One button per todo document, in the order given. A document with nothing open has no count.
 * @param {TodoDocument[] | null | undefined} documents @returns {{path: string, title: string, count: string}[]} */
export function todoButtons(documents) {
  return (documents ?? []).map(({ path, title, open }) => ({ path, title, count: open > 0 ? todoLabel(open) : "" }));
}
