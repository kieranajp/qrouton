// The panel's thoughts folders.

/**
 * @typedef {object} ThoughtsRow
 * @property {string} id
 * @property {string} path
 * @property {string} orgs
 * @property {boolean} [saved]
 */

/**
 * @typedef {object} ThoughtsForm
 * @property {string} default
 * @property {ThoughtsRow[]} roots
 */

const same = (a = "", b = "") => a.trim().replace(/\/+$/, "") === b.trim().replace(/\/+$/, "");

/** fromView marks every loaded row saved, which is what fixes its name.
 * @param {{default?: string, roots?: ThoughtsRow[]} | undefined} view
 * @returns {ThoughtsForm} */
export const fromView = (view) => ({
  default: view?.default ?? "",
  roots: (view?.roots ?? []).map(({ id, path, orgs }) => ({ id, path, orgs, saved: true })),
});

/** @param {ThoughtsForm} form */
export const toInput = (form) => ({
  default: form.default,
  roots: form.roots.map(({ id, path, orgs }) => ({ id, path, orgs })),
});

/** @param {ThoughtsForm} form */
export const addRow = (form) => ({
  ...form,
  roots: [...form.roots, { id: "", path: "", orgs: "", saved: false }],
});

/** @param {ThoughtsForm} form @param {number} index */
export const removeRow = (form, index) => ({ ...form, roots: form.roots.filter((_, i) => i !== index) });

/** movedPaths answers whether a session already writing somewhere loses it: a
 * changed default, a changed path, or a removed folder. Orgs and new rows move nothing.
 * @param {ThoughtsForm & {derived?: string}} loaded
 * @param {ThoughtsForm} form */
export function movedPaths(loaded, form) {
  const derived = loaded.derived ?? "";
  if (!same(loaded.default || derived, form.default || derived)) return true;
  return loaded.roots.some((was) => {
    const now = form.roots.find((row) => row.saved && row.id === was.id);
    return !now || !same(was.path, now.path);
  });
}
