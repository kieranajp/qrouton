const fence = "---";
const codeFence = /^(?:```|~~~)/;

/** The one-based source line range of each slide. A `---` opens one only after
 * a blank line and outside a code fence, so a setext underline is not a break.
 * @param {string} markdown
 * @returns {{line: number, lineEnd: number}[]} */
export function slideSpans(markdown) {
  const lines = (markdown ?? "").split("\n");
  // A file ending in a newline splits to a phantom last element, and counting it
  // would put a card's last line past the end of the document.
  if (lines.at(-1) === "") lines.pop();
  const start = frontmatterEnd(lines);
  const breaks = [];
  let fenced = false;
  for (let at = start; at < lines.length; at += 1) {
    const trimmed = lines[at].trim();
    if (codeFence.test(trimmed)) {
      fenced = !fenced;
    } else if (!fenced && trimmed === fence && (at === start || lines[at - 1].trim() === "")) {
      breaks.push(at);
    }
  }
  const spans = [];
  let from = start;
  for (const at of [...breaks, lines.length]) {
    if (lines.slice(from, at).some((line) => line.trim() !== "")) {
      spans.push({ line: from + 1, lineEnd: Math.max(from + 1, at) });
    }
    from = at + 1;
  }
  return spans;
}

// A block that never closes leaves no document, matching how Go reads the same
// frontmatter when it decides the file is a deck at all.
function frontmatterEnd(lines) {
  for (let at = 0; at < lines.length; at += 1) {
    const trimmed = lines[at].trim();
    if (trimmed === "") continue;
    if (trimmed !== fence) return at;
    for (let closing = at + 1; closing < lines.length; closing += 1) {
      if (lines[closing].trim() === fence) return closing + 1;
    }
    return lines.length;
  }
  return lines.length;
}

const assetRoot = "/deck";
const scheme = /^[a-z][a-z0-9+.-]*:/i;
const attribute = /(<(?:img|video|source)\b[^>]*?\bsrc\s*=\s*)(["'])([^"']*)\2/gi;
const inline = /(!\[[^\]]*\]\()\s*([^)\s]+)/g;

/** Deck media addressed at the workbench's own asset route, which serves the
 * deck's directory and only its media.
 * @param {string} markdown
 * @param {string | undefined} token */
export function deckAssets(markdown, token) {
  const source = markdown ?? "";
  if (!token) return source;
  const base = `${assetRoot}/${token}/`;
  return source
    .replace(attribute, (_, open, quote, url) => `${open}${quote}${routed(url, base)}${quote}`)
    .replace(inline, (_, open, url) => `${open}${routed(url, base)}`);
}

// An author's own absolute URL is theirs to keep; anything session-relative is
// meaningless to a webview until it names the route.
function routed(url, base) {
  if (url === "" || url.startsWith("/") || url.startsWith("#") || scheme.test(url)) return url;
  return base + url.replace(/^\.\//, "");
}
