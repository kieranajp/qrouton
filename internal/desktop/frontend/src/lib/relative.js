// How long ago something happened, kept pure: node --test is the whole
// frontend harness.

const MINUTE = 60000;
const HOUR = 60 * MINUTE;
const DAY = 24 * HOUR;
// Past a quarter, a count of days stops telling you anything a date would not.
const DATED = 90 * DAY;

/** @typedef {'compact'|'prose'} Style */

const STYLES = {
  compact: { minutes: true, dated: false, days: (count) => `${count}d ago` },
  prose: {
    minutes: false,
    dated: true,
    days: (count) => `${count} day${count === 1 ? "" : "s"} ago`,
  },
};

const WEEK = 7 * DAY;
const MONTH = 30 * DAY;

/**
 * age is the same reading with the words taken out, for a column that has room
 * for a number and a letter. It coarsens as it goes — minutes, hours, days,
 * weeks, months — because a rail row is chosen on rough recency.
 * @param {string|number|Date} at
 * @param {number} [now]
 */
export function age(at, now = Date.now()) {
  const then = new Date(at).getTime();
  if (!Number.isFinite(then) || then <= 0) return "";
  const since = now - then;
  if (since < MINUTE) return "now";
  if (since < HOUR) return `${Math.floor(since / MINUTE)}m`;
  if (since < DAY) return `${Math.floor(since / HOUR)}h`;
  if (since < WEEK) return `${Math.floor(since / DAY)}d`;
  if (since < MONTH) return `${Math.floor(since / WEEK)}w`;
  return `${Math.floor(since / MONTH)}mo`;
}

/**
 * relative is an age in words, never more precise than it is. A compact age
 * counts minutes and stays a count however old it gets; a prose age spells its
 * days out and carries a date once counting them says nothing. Something with
 * no time behind it says nothing rather than dating itself to the epoch.
 * @param {string|number|Date} at
 * @param {Style} style
 * @param {number} [now]
 */
export function relative(at, style, now = Date.now()) {
  const voice = STYLES[style] ?? STYLES.compact;
  const then = new Date(at).getTime();
  if (!Number.isFinite(then) || then <= 0) return "";
  const since = now - then;
  if (voice.dated && since >= DATED) return new Date(then).toISOString().slice(0, 10);
  if (since < HOUR) {
    const minutes = Math.floor(since / MINUTE);
    return voice.minutes && minutes >= 1 ? `${minutes}m ago` : "just now";
  }
  if (since < DAY) return `${Math.floor(since / HOUR)}h ago`;
  return voice.days(Math.floor(since / DAY));
}
