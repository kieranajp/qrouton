import test from "node:test";
import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import postcss from "postcss";

const src = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "../src");

const exemptFiles = new Set(["tokens/effects.css", "lib/panes/slide-theme.css"]);
const exemptProperties = new Set(["--terminal-size", "--w-traffic-lights", "box-shadow", "outline"]);
const deckStage = new Set(["1280", "720"]);
const exemptValues = new Map([
  ["lib/panes/SlidesPane.svelte", deckStage],
  ["lib/panes/PresentOverlay.svelte", deckStage],
]);

const scaled = /calc\(\s*-?\d*\.?\d+px \* var\(--ui-scale\)\s*\)/g;
const pixels = /(?<![\w.-])-?(\d*\.?\d+)px\b/g;

function files(dir) {
  return fs.readdirSync(dir, { withFileTypes: true }).flatMap((entry) => {
    const full = path.join(dir, entry.name);
    if (entry.isDirectory()) return files(full);
    return /\.(css|svelte)$/.test(entry.name) ? [full] : [];
  });
}

function styles(file, source) {
  if (file.endsWith(".css")) return [{ css: source, line: 0 }];
  return [...source.matchAll(/<style[^>]*>([\s\S]*?)<\/style>/g)]
    .filter((match) => !/^\s*\$?\{/.test(match[1]))
    .map((match) => ({ css: match[1], line: source.slice(0, match.index).split("\n").length - 1 }));
}

/** @param {string} value @param {Set<string> | undefined} allowed */
function unscaled(value, allowed) {
  return [...value.replace(scaled, "").matchAll(pixels)]
    .filter((match) => Number(match[1]) > 2 && !allowed?.has(match[1]))
    .map((match) => match[0]);
}

function violations() {
  const found = [];
  for (const file of files(src)) {
    const rel = path.relative(src, file).split(path.sep).join("/");
    if (exemptFiles.has(rel)) continue;
    for (const block of styles(file, fs.readFileSync(file, "utf8"))) {
      const root = postcss.parse(block.css);
      const report = (node, text) => {
        for (const px of unscaled(text, exemptValues.get(rel))) {
          found.push(`${rel}:${block.line + (node.source?.start?.line ?? 0)}: ${px}`);
        }
      };
      root.walkDecls((decl) => {
        if (!exemptProperties.has(decl.prop)) report(decl, decl.value);
      });
      root.walkAtRules((rule) => report(rule, rule.params));
    }
  }
  return found;
}

test("app CSS scales every length over 2px with --ui-scale", () => {
  assert.deepEqual(violations(), []);
});

test("the guard accepts the scaled form and refuses a bare length", () => {
  assert.deepEqual(unscaled("calc(12px * var(--ui-scale)) 1px", undefined), []);
  assert.deepEqual(unscaled("calc(100% - calc(24px * var(--ui-scale)))", undefined), []);
  assert.deepEqual(unscaled("0 12px", undefined), ["12px"]);
  assert.deepEqual(unscaled("-6px", undefined), ["-6px"]);
  assert.deepEqual(unscaled("calc(100% - 24px)", undefined), ["24px"]);
});
