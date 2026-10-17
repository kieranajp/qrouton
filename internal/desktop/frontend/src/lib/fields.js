const patterns = new WeakMap();

/** Text that names no listed field answers null instead of guessing one.
 * @param {any} err
 * @param {Record<string, string>} fields
 * @param {string[]} [shapes] Regexp sources for generated names that carry an index. */
export function fieldError(err, fields, shapes = []) {
  let pattern = patterns.get(fields);
  if (!pattern) {
    pattern = new RegExp(`^(${[...shapes, ...Object.values(fields)].join("|")}): (.*)$`, "s");
    patterns.set(fields, pattern);
  }
  const found = String(err?.message ?? err ?? "").match(pattern);
  return found ? { field: found[1], message: found[2] } : null;
}
