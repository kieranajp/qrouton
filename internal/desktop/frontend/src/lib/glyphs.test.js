import assert from "node:assert/strict";
import { readdirSync, readFileSync } from "node:fs";
import { join, relative } from "node:path";
import test from "node:test";

const SRC = join(import.meta.dirname, "..");

const GLYPHS = [..."✕×⌕▾▸▲▼›‹↻⟳←→↑↓✓✔✗✖◌●◐◆▶", "🔔"];

const NAMED = {
  times: "×",
  rsaquo: "›",
  lsaquo: "‹",
  larr: "←",
  rarr: "→",
  uarr: "↑",
  darr: "↓",
  check: "✓",
  checkmark: "✓",
  cross: "✗",
  rtrif: "▸",
  dtrif: "▾",
  utrif: "▴",
};

const ALLOWED = ["Used by Work on issue → Custom script."];

const decode = (text) =>
  text
    .replace(/\\u\{?([0-9a-f]{4,5})\}?/gi, (_, hex) => String.fromCodePoint(parseInt(hex, 16)))
    .replace(/&#x([0-9a-f]+);/gi, (_, hex) => String.fromCodePoint(parseInt(hex, 16)))
    .replace(/&#(\d+);/g, (_, dec) => String.fromCodePoint(Number(dec)))
    .replace(/&([a-z]+);/gi, (entity, name) => NAMED[name] ?? entity);

function* sources(dir) {
  for (const entry of readdirSync(dir, { withFileTypes: true })) {
    const path = join(dir, entry.name);
    if (entry.isDirectory()) yield* sources(path);
    else if (/\.(svelte|js|css)$/.test(entry.name) && !entry.name.endsWith(".test.js")) yield path;
  }
}

test("no chrome glyph the Mitre set replaced comes back", () => {
  const found = [];
  for (const path of sources(SRC)) {
    if (path.endsWith("SessionCard.svelte")) continue;
    readFileSync(path, "utf8")
      .split("\n")
      .forEach((line, i) => {
        if (ALLOWED.some((allowed) => line.includes(allowed))) return;
        const text = decode(line);
        const glyph = GLYPHS.find((g) => text.includes(g));
        if (glyph) found.push(`${relative(SRC, path)}:${i + 1} ${glyph}`);
      });
  }
  assert.deepEqual(found, []);
});
