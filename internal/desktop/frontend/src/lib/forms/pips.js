// Pip progress, kept pure: node --test is the whole frontend harness.

/**
 * clamp keeps an active index inside the pips there are to draw, so a step past
 * the end still lights one.
 * @param {number} total
 * @param {number} active
 */
const clamp = (total, active) => Math.min(Math.max(Math.trunc(active) || 0, 0), Math.max(total - 1, 0));

/**
 * pipStates is what each pip draws as, in order.
 * @param {number} total
 * @param {number} [active]
 * @returns {('done'|'on'|'todo')[]}
 */
export function pipStates(total, active = 0) {
  const on = clamp(total, active);
  return Array.from({ length: Math.max(total, 0) }, (_, i) =>
    i === on ? "on" : i < on ? "done" : "todo",
  );
}

/**
 * pipCounter says which of them is on, counting from one.
 * @param {number} total
 * @param {number} [active]
 */
export const pipCounter = (total, active = 0) => `${clamp(total, active) + 1} of ${total}`;
