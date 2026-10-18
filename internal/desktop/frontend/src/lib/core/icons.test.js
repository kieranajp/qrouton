import assert from "node:assert/strict";
import { readdirSync, readFileSync } from "node:fs";
import { join, relative } from "node:path";
import test from "node:test";
import { REPO_ROLES, TAB_STATUSES, WINDOW_ICONS } from "../bridge/generated.js";
import { ICONS } from "./icons.js";

const SRC = join(import.meta.dirname, "..", "..");

const LITERAL_NAMES = [
  /<Icon\b[^>]*?\bname="([^"{]+)"/g,
  /\b(?:icon|iconEnd|metaIcon)="([^"{]+)"/g,
  /\bicon: "([^"]+)"/g,
];

function* sources(dir) {
  for (const entry of readdirSync(dir, { withFileTypes: true })) {
    const path = join(dir, entry.name);
    if (entry.isDirectory()) yield* sources(path);
    else if (/\.(svelte|js)$/.test(entry.name) && !entry.name.endsWith(".test.js")) yield path;
  }
}

test("every literal icon name in the source is in the set", () => {
  const missing = [];
  let seen = 0;
  for (const path of sources(SRC)) {
    const text = readFileSync(path, "utf8");
    for (const pattern of LITERAL_NAMES) {
      for (const [, name] of text.matchAll(pattern)) {
        seen++;
        if (!(name in ICONS)) missing.push(`${relative(SRC, path)}: ${name}`);
      }
    }
  }
  assert.ok(seen > 0, "the scan found no icon names");
  assert.deepEqual(missing, []);
});

test("every bridge value drawn as an icon is in the set", () => {
  for (const set of [WINDOW_ICONS, TAB_STATUSES, REPO_ROLES]) {
    for (const name of Object.values(set)) {
      assert.ok(name in ICONS, `${name} has no icon`);
    }
  }
});

test("every icon draws something", () => {
  for (const [name, { stroke, fill }] of Object.entries(ICONS)) {
    assert.ok(stroke || fill, `${name} has no path`);
  }
});

test("no icon bends a line", () => {
  for (const [name, { stroke, fill }] of Object.entries(ICONS)) {
    for (const path of [stroke, fill].filter(Boolean)) {
      assert.doesNotMatch(path, /[cCsSqQtTaA]/, `${name} has a curve`);
    }
  }
});
