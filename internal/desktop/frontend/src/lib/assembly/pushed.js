// When a repository was last pushed to, as its row says it, kept pure:
// node --test is the whole frontend harness.

const HOUR = 3600000;
const DAY = 24 * HOUR;
// Past a quarter, a count of days stops telling you anything a date would not.
const DATED = 90 * DAY;

/**
 * pushed is a repository row's meta line. A repository with no push behind it
 * says nothing rather than dating itself to the epoch.
 * @param {string|number|Date} at
 * @param {number} [now]
 */
export function pushed(at, now = Date.now()) {
  const then = new Date(at).getTime();
  if (!Number.isFinite(then) || then <= 0) return "";
  const since = now - then;
  if (since >= DATED) return "pushed " + new Date(then).toISOString().slice(0, 10);
  const days = Math.floor(since / DAY);
  if (days >= 1) return `pushed ${days} day${days === 1 ? "" : "s"} ago`;
  const hours = Math.floor(since / HOUR);
  if (hours >= 1) return `pushed ${hours}h ago`;
  return "pushed just now";
}
