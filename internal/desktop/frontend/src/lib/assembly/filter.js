// Narrowing the repository list, kept pure: node --test is the whole frontend
// harness.

/** @typedef {{org: string, name: string, default_branch?: string, pushed_at?: string}} Repo */
/** @typedef {Repo & {id: string}} Row */

/** repoID is the key a repository is known by: the `org/name` Go matches on too. */
export const repoID = (repo) => repo.org + "/" + repo.name;

/**
 * filter produces the rows step 2 draws and the count above them together, so
 * `6 of 412 shown` cannot disagree with the list under it. The total is the whole
 * cache; only the shown answers to the cap. Pinned ids sort first — they are the
 * session's own repositories, which a cap counted in activity order would
 * otherwise hide behind repos it has never touched.
 * @param {{repos?: Repo[], owners?: string[], query?: string, cap?: number, pinned?: string[]}} [options]
 * @returns {{rows: Row[], shown: number, total: number}}
 */
export function filter({ repos = [], owners = [], query = "", cap = 0, pinned = [] } = {}) {
  const included = new Set(owners);
  const first = new Set(pinned);
  const needle = query.trim().toLowerCase();
  const held = [];
  const rest = [];
  for (const repo of repos) {
    if (!included.has(repo.org)) continue;
    const id = repoID(repo);
    if (needle && !id.toLowerCase().includes(needle)) continue;
    (first.has(id) ? held : rest).push({ ...repo, id });
  }
  const matched = [...held, ...rest];
  const rows = cap > 0 ? matched.slice(0, cap) : matched;
  return { rows, shown: rows.length, total: repos.length };
}
