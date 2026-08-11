const META = [
  "diff --git",
  "index ",
  "--- ",
  "+++ ",
  "=== ",
  "new file",
  "deleted file",
  "rename ",
  "similarity ",
  "old mode",
  "new mode",
  "Binary files",
];

/**
 * diffClass names one line's part in a unified diff.
 * @param {string} line
 * @returns {string}
 */
export function diffClass(line) {
  if (META.some((prefix) => line.startsWith(prefix))) return "diff-file";
  if (line.startsWith("@@")) return "diff-hunk";
  if (line.startsWith("+")) return "diff-add";
  if (line.startsWith("-")) return "diff-del";
  return "";
}
