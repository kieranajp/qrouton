/** @typedef {{text: string, section: string, where: string, path: string, line: number}} TodoItem */

/** The For you tier's rows, in document order.
 * @param {TodoItem[] | null | undefined} items @returns {{key: string, item: TodoItem}[]} */
export function todoTier(items) {
  return (items ?? []).map((item) => ({ key: `${item.path}:${item.line}`, item }));
}
