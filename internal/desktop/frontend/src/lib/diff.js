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

const HUNK = /^@@ -([0-9]+)(?:,([0-9]+))? \+([0-9]+)(?:,([0-9]+))? @@(?: .*)?$/;

/** @typedef {"file" | "hunk" | "add" | "del" | "context" | "marker" | "plain"} DiffKind */

/**
 * @param {string} line
 * @returns {DiffKind}
 */
function outsideKind(line) {
  if (META.some((prefix) => line.startsWith(prefix))) return "file";
  if (line.startsWith("@@")) return "hunk";
  if (line.startsWith("+")) return "add";
  if (line.startsWith("-")) return "del";
  return "plain";
}

/**
 * @param {number} start
 * @param {number} count
 */
function safeRange(start, count) {
  return Number.isSafeInteger(start)
    && Number.isSafeInteger(count)
    && (count === 0 || start <= Number.MAX_SAFE_INTEGER - (count - 1));
}

/**
 * @param {string} line
 * @returns {{oldStart: number, oldCount: number, newStart: number, newCount: number} | null}
 */
function readHeader(line) {
  const match = HUNK.exec(line);
  if (!match) return null;

  const oldStart = Number(match[1]);
  const oldCount = match[2] === undefined ? 1 : Number(match[2]);
  const newStart = Number(match[3]);
  const newCount = match[4] === undefined ? 1 : Number(match[4]);
  if (!safeRange(oldStart, oldCount) || !safeRange(newStart, newCount)) return null;

  return { oldStart, oldCount, newStart, newCount };
}

/**
 * @param {string} text
 * @returns {{rows: Array<{text: string, kind: DiffKind, oldLine: number | null, newLine: number | null}>, digits: number}}
 */
export function parseDiff(text) {
  /** @type {Array<{text: string, kind: DiffKind, oldLine: number | null, newLine: number | null}>} */
  const rows = [];
  /** @type {null | {oldStart: number, oldCount: number, newStart: number, newCount: number, oldUsed: number, newUsed: number, rows: Array<{text: string, kind: DiffKind, oldOffset: number | null, newOffset: number | null}>}} */
  let candidate = null;

  const flush = () => {
    if (candidate === null) return;
    const valid = candidate.oldUsed === candidate.oldCount
      && candidate.newUsed === candidate.newCount;
    for (const row of candidate.rows) {
      rows.push({
        text: row.text,
        kind: row.kind,
        oldLine: valid && row.oldOffset !== null ? candidate.oldStart + row.oldOffset : null,
        newLine: valid && row.newOffset !== null ? candidate.newStart + row.newOffset : null,
      });
    }
  };

  for (const line of text.split("\n")) {
    let handled = false;
    while (!handled) {
      if (candidate !== null) {
        if (line === "\\ No newline at end of file") {
          candidate.rows.push({
            text: line,
            kind: "marker",
            oldOffset: null,
            newOffset: null,
          });
          handled = true;
        } else if (line.startsWith(" ")) {
          candidate.rows.push({
            text: line,
            kind: "context",
            oldOffset: candidate.oldUsed,
            newOffset: candidate.newUsed,
          });
          candidate.oldUsed++;
          candidate.newUsed++;
          handled = true;
        } else if (line.startsWith("-")) {
          candidate.rows.push({
            text: line,
            kind: "del",
            oldOffset: candidate.oldUsed,
            newOffset: null,
          });
          candidate.oldUsed++;
          handled = true;
        } else if (line.startsWith("+")) {
          candidate.rows.push({
            text: line,
            kind: "add",
            oldOffset: null,
            newOffset: candidate.newUsed,
          });
          candidate.newUsed++;
          handled = true;
        } else {
          flush();
          candidate = null;
        }
        continue;
      }

      const header = readHeader(line);
      if (header !== null) {
        candidate = {
          ...header,
          oldUsed: 0,
          newUsed: 0,
          rows: [{
            text: line,
            kind: "hunk",
            oldOffset: null,
            newOffset: null,
          }],
        };
      } else {
        rows.push({
          text: line,
          kind: outsideKind(line),
          oldLine: null,
          newLine: null,
        });
      }
      handled = true;
    }
  }
  flush();

  let digits = 1;
  for (const row of rows) {
    if (row.oldLine !== null) digits = Math.max(digits, String(row.oldLine).length);
    if (row.newLine !== null) digits = Math.max(digits, String(row.newLine).length);
  }
  return { rows, digits };
}
