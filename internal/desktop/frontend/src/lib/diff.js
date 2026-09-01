const META = [
  "diff --git", "index ", "--- ", "+++ ", "=== ", "new file", "deleted file",
  "rename ", "copy ", "similarity ", "dissimilarity ", "old mode", "new mode", "Binary files",
];

const HUNK = /^@@ -([0-9]+)(?:,([0-9]+))? \+([0-9]+)(?:,([0-9]+))? @@(?: .*)?$/;
const REPOSITORY = /^=== (.+) ===$/;
const NO_CHANGES = /^No changes in (.+)\.$/;
const MODE = /^[0-7]{6}$/;

/** @typedef {"file" | "hunk" | "add" | "del" | "context" | "marker" | "plain"} DiffKind */
/** @typedef {"full" | "partial" | "none"} PatchConfidence */
/** @typedef {{id: string, name: string, label: string, from: number, to: number, framingFrom: number, framingTo: number, fileIds: string[], filesChanged: number, zeroChange: boolean, confidence: PatchConfidence}} PatchRepository */

function outsideKind(line) {
  if (META.some((prefix) => line.startsWith(prefix))) return "file";
  if (line.startsWith("@@")) return "hunk";
  if (line.startsWith("+")) return "add";
  if (line.startsWith("-")) return "del";
  return "plain";
}

function safeRange(start, count) {
  return Number.isSafeInteger(start) && Number.isSafeInteger(count)
    && (count === 0 || start <= Number.MAX_SAFE_INTEGER - (count - 1));
}

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

function sourceLines(text) {
  const lines = [];
  let from = 0;
  for (let index = 0; index <= text.length; index++) {
    if (index !== text.length && text.charCodeAt(index) !== 10) continue;
    const raw = text.slice(from, index);
    lines.push({
      from,
      to: index < text.length ? index + 1 : index,
      raw,
      text: raw.endsWith("\r") ? raw.slice(0, -1) : raw,
    });
    from = index + 1;
  }
  return lines;
}

function scanHunks(lines, visit) {
  let candidate = null;
  const hunks = [];
  const flush = () => {
    if (candidate === null) return;
    const valid = candidate.oldUsed === candidate.oldCount && candidate.newUsed === candidate.newCount;
    for (const row of candidate.rows) {
      visit(row.line, row.kind, {
        valid,
        oldLine: valid && row.oldOffset !== null ? candidate.oldStart + row.oldOffset : null,
        newLine: valid && row.newOffset !== null ? candidate.newStart + row.newOffset : null,
      });
    }
    hunks.push({
      from: candidate.rows[0].line.from,
      to: candidate.rows.at(-1).line.to,
      valid,
      additions: valid ? candidate.newUsed - candidate.context : null,
      deletions: valid ? candidate.oldUsed - candidate.context : null,
    });
  };

  for (const line of lines) {
    let handled = false;
    while (!handled) {
      if (candidate !== null) {
        if (line.text === "\\ No newline at end of file") {
          candidate.rows.push({ line, kind: "marker", oldOffset: null, newOffset: null });
          handled = true;
        } else if (line.text.startsWith(" ")) {
          candidate.rows.push({ line, kind: "context", oldOffset: candidate.oldUsed, newOffset: candidate.newUsed });
          candidate.oldUsed++;
          candidate.newUsed++;
          candidate.context++;
          handled = true;
        } else if (line.text.startsWith("-")) {
          candidate.rows.push({ line, kind: "del", oldOffset: candidate.oldUsed, newOffset: null });
          candidate.oldUsed++;
          handled = true;
        } else if (line.text.startsWith("+")) {
          candidate.rows.push({ line, kind: "add", oldOffset: null, newOffset: candidate.newUsed });
          candidate.newUsed++;
          handled = true;
        } else {
          flush();
          candidate = null;
        }
        continue;
      }

      const header = readHeader(line.text);
      if (header !== null) {
        candidate = {
          ...header,
          oldUsed: 0,
          newUsed: 0,
          context: 0,
          rows: [{ line, kind: "hunk", oldOffset: null, newOffset: null }],
        };
      } else {
        visit(line, outsideKind(line.text), { valid: null, oldLine: null, newLine: null });
      }
      handled = true;
    }
  }
  flush();
  return hunks;
}

export function parseDiff(text) {
  const rows = [];
  scanHunks(sourceLines(text), (line, kind, position) => {
    rows.push({ text: line.raw, kind, oldLine: position.oldLine, newLine: position.newLine });
  });
  let digits = 1;
  for (const row of rows) {
    if (row.oldLine !== null) digits = Math.max(digits, String(row.oldLine).length);
    if (row.newLine !== null) digits = Math.max(digits, String(row.newLine).length);
  }
  return { rows, digits };
}

function readQuotedToken(source, start) {
  let index = start + 1;
  const bytes = [];
  let visible = "";
  const encoder = new TextEncoder();
  const named = new Map([
    ["a", 7], ["b", 8], ["t", 9], ["n", 10], ["v", 11], ["f", 12], ["r", 13],
    ["\\", 92], ["\"", 34],
  ]);
  while (index < source.length && source[index] !== "\"") {
    if (source[index] !== "\\") {
      const character = String.fromCodePoint(source.codePointAt(index));
      bytes.push(...encoder.encode(character));
      visible += character;
      index += character.length;
      continue;
    }
    const escapeStart = index++;
    if (index >= source.length) return null;
    if (named.has(source[index])) {
      bytes.push(named.get(source[index]));
      visible += source.slice(escapeStart, index + 1);
      index++;
      continue;
    }
    const octal = source.slice(index, index + 3);
    if (!/^[0-7]{3}$/.test(octal)) return null;
    bytes.push(Number.parseInt(octal, 8));
    visible += source.slice(escapeStart, index + 3);
    index += 3;
  }
  if (source[index] !== "\"") return null;
  let value = visible;
  try {
    value = new TextDecoder("utf-8", { fatal: true }).decode(new Uint8Array(bytes));
  } catch {
    // Keep the escaped spelling visible when the quoted bytes are not UTF-8.
  }
  return { value, end: index + 1 };
}

function readToken(source, start) {
  if (source[start] === "\"") return readQuotedToken(source, start);
  let end = start;
  while (end < source.length && source[end] !== " " && source[end] !== "\t") end++;
  if (end === start || /[\\"\u0000-\u001f\u007f]/.test(source.slice(start, end))) return null;
  return { value: source.slice(start, end), end };
}

function readDiffBoundary(line) {
  if (!line.startsWith("diff --git ")) return null;
  let index = "diff --git ".length;
  const oldToken = readToken(line, index);
  if (oldToken === null) return null;
  index = oldToken.end;
  if (line[index] !== " ") return null;
  while (line[index] === " ") index++;
  const newToken = readToken(line, index);
  if (newToken === null || newToken.end !== line.length) return null;
  if (!oldToken.value.startsWith("a/") || !newToken.value.startsWith("b/")) return null;
  return { oldPath: oldToken.value, newPath: newToken.value };
}

function readMetadataPath(value) {
  if (value.startsWith("\"")) {
    const token = readQuotedToken(value, 0);
    return token !== null && token.end === value.length ? token.value : null;
  }
  return value !== "" && !/[\u0000\r\n]/.test(value) ? value : null;
}

function displayPath(path, prefix) {
  if (path === "/dev/null") return null;
  return path.startsWith(prefix) ? path.slice(prefix.length) : path;
}

function repositoryName(label) {
  const trimmed = label.replace(/[\\/]+$/, "");
  return trimmed.split(/[\\/]/).at(-1) || label;
}

function metadata(line) {
  if (/^index [0-9a-fA-F]+\.\.[0-9a-fA-F]+(?: [0-7]{6})?$/.test(line)) return { kind: "index" };
  for (const kind of ["new file mode", "deleted file mode", "old mode", "new mode"]) {
    if (line.startsWith(`${kind} `) && MODE.test(line.slice(kind.length + 1))) return { kind };
  }
  for (const kind of ["similarity index", "dissimilarity index"]) {
    const match = new RegExp(`^${kind} ([0-9]{1,3})%$`).exec(line);
    if (match && Number(match[1]) <= 100) return { kind };
  }
  for (const kind of ["rename from", "rename to", "copy from", "copy to"]) {
    if (!line.startsWith(`${kind} `)) continue;
    const path = readMetadataPath(line.slice(kind.length + 1));
    return path === null ? null : { kind, path };
  }
  for (const [kind, prefix] of [["---", "--- "], ["+++", "+++ "]]) {
    if (!line.startsWith(prefix)) continue;
    const path = readMetadataPath(line.slice(prefix.length));
    return path === null ? null : { kind, path };
  }
  if (/^Binary files .+ and .+ differ$/.test(line)) return { kind: "binary" };
  return null;
}

function appendRegion(regions, kind, from, to, details = {}) {
  if (to <= from) return;
  const previous = regions.at(-1);
  if (previous && previous.kind === kind && previous.to === from
      && previous.fileId === details.fileId && previous.repositoryId === details.repositoryId
      && previous.role === details.role) {
    previous.to = to;
    return;
  }
  regions.push({ kind, from, to, ...details });
}

function repositoryFrames(lines) {
  const frames = [];
  for (let index = 0; index < lines.length; index++) {
    const match = REPOSITORY.exec(lines[index].text);
    if (!match || /[\u0000-\u001f\u007f]/.test(match[1])) continue;
    let from = lines[index].from;
    let startLineIndex = index;
    if (index > 0 && lines[index - 1].text === "" && lines[index - 1].to === from) {
      from = lines[index - 1].from;
      startLineIndex = index - 1;
    }
    frames.push({ from, to: lines[index].to, startLineIndex, lineIndex: index, label: match[1] });
  }
  return frames;
}

function classifyFile(lines, boundary, endLineIndex, nextBoundary, repository) {
  const slice = lines.slice(boundary.lineIndex, endLineIndex);
  const recognized = new Map([[boundary.line.from, true]]);
  const metadataByOffset = new Map();
  const hunks = scanHunks(slice, (line, _kind, position) => {
    if (position.valid !== null) recognized.set(line.from, position.valid);
  });
  for (const line of slice.slice(1)) {
    if (recognized.has(line.from)) continue;
    const parsed = metadata(line.text);
    recognized.set(line.from, parsed !== null);
    if (parsed !== null) metadataByOffset.set(line.from, parsed);
  }

  const values = [...metadataByOffset.values()];
  const kinds = new Set(values.map((value) => value.kind));
  const completePairs = [
    ["rename from", "rename to"],
    ["copy from", "copy to"],
    ["old mode", "new mode"],
    ["---", "+++"],
  ];
  for (const [left, right] of completePairs) {
    if (kinds.has(left) === kinds.has(right)) continue;
    for (const [offset, value] of metadataByOffset) {
      if (value.kind === left || value.kind === right) recognized.set(offset, false);
    }
  }
  const rename = values.find((value) => value.kind === "rename to");
  const copy = values.find((value) => value.kind === "copy to");
  const oldPath = displayPath(boundary.paths.oldPath, "a/");
  const newPath = displayPath(boundary.paths.newPath, "b/");
  let status = "unknown";
  if (kinds.has("binary")) status = "binary";
  else if (kinds.has("rename from") && rename) status = "renamed";
  else if (kinds.has("copy from") && copy) status = "copied";
  else if (kinds.has("new file mode") || oldPath === null) status = "added";
  else if (kinds.has("deleted file mode") || newPath === null) status = "deleted";
  else if (kinds.has("old mode") && kinds.has("new mode") && hunks.length === 0) status = "mode-only";
  else if (hunks.some((hunk) => hunk.valid)) status = "modified";

  const allRecognized = [...recognized.values()].every(Boolean);
  const validHunks = hunks.filter((hunk) => hunk.valid);
  const exactZero = ["added", "deleted", "renamed", "copied", "mode-only"].includes(status) && hunks.length === 0;
  const countsAvailable = allRecognized && !kinds.has("binary") && (validHunks.length > 0 || exactZero);
  const additions = countsAvailable ? validHunks.reduce((sum, hunk) => sum + hunk.additions, 0) : null;
  const deletions = countsAvailable ? validHunks.reduce((sum, hunk) => sum + hunk.deletions, 0) : null;
  const path = status === "deleted" ? oldPath : (rename?.path ?? copy?.path ?? newPath ?? oldPath ?? boundary.paths.newPath);
  const id = `file:${repository?.id ?? "single"}:${boundary.line.from}`;
  return {
    file: {
      id,
      repositoryId: repository?.id ?? null,
      repository: repository?.name ?? null,
      from: boundary.line.from,
      to: nextBoundary,
      contentFrom: boundary.line.from,
      contentTo: nextBoundary,
      headerFrom: boundary.line.from,
      path,
      oldPath,
      newPath,
      status,
      additions,
      deletions,
      countsAvailable,
      confidence: allRecognized ? "full" : "partial",
    },
    lines: slice,
    recognized,
  };
}

export function parsePatch(raw) {
  const lines = sourceLines(raw);
  const contentLineCount = lines.at(-1)?.from === raw.length ? lines.length - 1 : lines.length;
  const frames = repositoryFrames(lines);
  /** @type {PatchRepository[]} */
  const repositories = frames.map((frame, index) => ({
    id: `repository:${frame.from}`,
    name: repositoryName(frame.label),
    label: frame.label,
    from: frame.from,
    to: frames[index + 1]?.from ?? raw.length,
    framingFrom: frame.from,
    framingTo: frame.to,
    fileIds: [],
    filesChanged: 0,
    zeroChange: false,
    confidence: "full",
  }));
  const boundaries = [];
  let repository = null;
  let repositoryIndex = -1;
  const frameLines = new Map(frames.map((frame, index) => [frame.lineIndex, index]));
  for (let index = 0; index < lines.length; index++) {
    if (frameLines.has(index)) {
      repositoryIndex = frameLines.get(index);
      repository = repositories[repositoryIndex];
      continue;
    }
    const paths = readDiffBoundary(lines[index].text);
    if (paths !== null) boundaries.push({ line: lines[index], lineIndex: index, paths, repository, repositoryIndex });
  }

  const files = [];
  const fileData = [];
  for (let index = 0; index < boundaries.length; index++) {
    const boundary = boundaries[index];
    const nextFile = boundaries[index + 1]?.line.from ?? raw.length;
    const nextFrame = frames[boundary.repositoryIndex + 1];
    const nextFrameFrom = nextFrame?.from ?? raw.length;
    const nextBoundary = Math.min(nextFile, nextFrameFrom);
    const endLineIndex = nextFile <= nextFrameFrom
      ? (boundaries[index + 1]?.lineIndex ?? contentLineCount)
      : nextFrame.startLineIndex;
    const data = classifyFile(lines, boundary, endLineIndex, nextBoundary, boundary.repository);
    files.push(data.file);
    fileData.push(data);
    if (boundary.repository) {
      boundary.repository.fileIds.push(data.file.id);
      boundary.repository.filesChanged++;
    }
  }

  const noChangeMatch = NO_CHANGES.exec(raw);
  const noChange = noChangeMatch === null ? null : { from: 0, to: raw.length, scope: noChangeMatch[1] };
  const regions = [];
  let cursor = 0;
  const events = [
    ...frames.map((frame, index) => ({ type: "frame", from: frame.from, to: frame.to, repository: repositories[index] })),
    ...fileData.flatMap((data) => data.lines.map((line) => ({
      type: data.recognized.get(line.from) ? "file" : "unassigned",
      from: line.from,
      to: line.to,
      file: data.file,
    }))),
  ].filter((event) => event.to > event.from).sort((left, right) => left.from - right.from || left.to - right.to);

  if (noChange !== null) {
    appendRegion(regions, "repository", 0, raw.length, { role: "no-change" });
    cursor = raw.length;
  } else {
    for (const event of events) {
      if (event.from < cursor) continue;
      if (event.from > cursor) appendRegion(regions, "unassigned", cursor, event.from);
      if (event.type === "frame") {
        appendRegion(regions, "repository", event.from, event.to, { repositoryId: event.repository.id, role: "framing" });
      } else if (event.type === "file") {
        appendRegion(regions, "file", event.from, event.to, { fileId: event.file.id });
      } else {
        appendRegion(regions, "unassigned", event.from, event.to, { fileId: event.file.id });
      }
      cursor = event.to;
    }
  }
  if (cursor < raw.length) appendRegion(regions, "unassigned", cursor, raw.length);

  const unassigned = regions.filter((region) => region.kind === "unassigned");
  for (const entry of repositories) {
    const hasUnassigned = unassigned.some((region) => region.from < entry.to && region.to > entry.framingTo);
    entry.zeroChange = entry.filesChanged === 0 && !hasUnassigned;
    entry.confidence = hasUnassigned ? "partial" : "full";
  }
  const available = files.length > 0 || noChange !== null || repositories.some((entry) => entry.zeroChange);
  const confidence = !available ? "none" : (unassigned.length === 0 ? "full" : "partial");
  const countsAvailable = available && unassigned.length === 0 && files.every((file) => file.countsAvailable);
  const totals = {
    files: files.length,
    additions: countsAvailable ? files.reduce((sum, file) => sum + file.additions, 0) : null,
    deletions: countsAvailable ? files.reduce((sum, file) => sum + file.deletions, 0) : null,
    available: countsAvailable,
  };
  return { raw, regions, repositories, files, totals, unassigned, confidence, available, noChange };
}
