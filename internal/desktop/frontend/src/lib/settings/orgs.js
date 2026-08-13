// The panel's org list, kept pure: node --test is the whole frontend harness.

/**
 * addOrg trims raw and adds it unless it is blank or already in the list.
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
 * removeOrg drops org from the list, leaving the rest of the order intact.
 * @param {string[]} list
 * @param {string} org
 * @returns {string[]}
 */
export const removeOrg = (list, org) => list.filter((seen) => seen !== org);
