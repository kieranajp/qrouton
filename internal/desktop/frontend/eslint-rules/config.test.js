import assert from "node:assert/strict";
import path from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";
import { ESLint } from "eslint";

test("JavaScript and Svelte receive the same active comment policy with their own parsers", async () => {
  const cwd = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
  const eslint = new ESLint({ cwd });
  const js = await eslint.calculateConfigForFile("src/lib/menu.js");
  const svelte = await eslint.calculateConfigForFile("src/lib/shell/Menu.svelte");
  assert.ok(Object.keys(js.rules).length > 0);
  assert.deepEqual(svelte.rules, js.rules);
  assert.notEqual(svelte.languageOptions.parser, js.languageOptions.parser);
  assert.equal(await eslint.isPathIgnored("src/lib/bridge/generated.js"), true);
});
