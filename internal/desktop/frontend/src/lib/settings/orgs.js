// The panel's org list, kept pure: node --test is the whole frontend harness.

/**
 * @param {string[]} list
 * @param {string} raw
 * @returns {string[]}
 */
export function addOrg(list, raw) {
  const org = raw.trim();
  if (!org || list.includes(org)) return list;
  return [...list, org];
}

/**
 * @param {string[]} list
 * @param {string} org
 * @returns {string[]}
 */
export const removeOrg = (list, org) => list.filter((seen) => seen !== org);
