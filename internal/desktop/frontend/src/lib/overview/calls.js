import { CHROME_AGENT_TREE, WINDOWS_OPEN_DOCUMENT_AT } from "../bridge/generated.js";
import { Call, call } from "../wails.js";

/** @param {string} slug */
export const agentTree = (slug) => call(Call.ByName(CHROME_AGENT_TREE, slug));

/** Opens a session document scrolled to a one-based line, or moves its open tab there.
 * @param {string} slug @param {string} path @param {number} line */
export const openDocumentAt = (slug, path, line) => call(Call.ByName(WINDOWS_OPEN_DOCUMENT_AT, slug, path, line));

/** Shows a session, then opens one of its documents at a line. A session that did not show gets nothing opened.
 * @param {(slug: string) => Promise<boolean | undefined> | undefined} show @param {string} slug @param {string} path @param {number} line */
export async function openTodo(show, slug, path, line) {
  if (!(await show(slug))) return;
  const opened = await openDocumentAt(slug, path, line);
  if (!opened.ok) console.error("open todo", path, opened.error);
}
