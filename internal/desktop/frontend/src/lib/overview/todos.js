/** @typedef {{text: string, section: string, where: string, blocks: string, path: string, line: number}} TodoItem */

/** The For you tier's rows: blocking items first, then the rest, each in document order.
 * @param {TodoItem[] | null | undefined} items @returns {{key: string, item: TodoItem, blocking: boolean}[]} */
export function todoTier(items) {
  const rows = (items ?? []).map((item) => ({ key: `${item.path}:${item.line}`, item, blocking: Boolean(item.blocks) }));
  return [...rows.filter((row) => row.blocking), ...rows.filter((row) => !row.blocking)];
}
