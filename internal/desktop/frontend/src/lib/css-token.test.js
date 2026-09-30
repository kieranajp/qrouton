import assert from "node:assert/strict";
import test from "node:test";
import { resolveToken } from "./css-token.js";

const reader = (declared) => (name) => declared[name] ?? "";

test("an alias resolves to the literal its chain ends in", () => {
  const read = reader({ "--surface-terminal": " var(--ctp-crust)", "--ctp-crust": "#181926" });
  assert.equal(resolveToken(read, "--surface-terminal"), "#181926");
});

test("a literal and an undeclared name answer as the engine does", () => {
  const read = reader({ "--ctp-crust": "#181926" });
  assert.equal(resolveToken(read, "--ctp-crust"), "#181926");
  assert.equal(resolveToken(read, "--nothing"), "");
});

test("a cycle stops rather than recursing forever", () => {
  const read = reader({ "--a": "var(--b)", "--b": "var(--a)" });
  assert.match(resolveToken(read, "--a"), /^var\(--[ab]\)$/);
});
