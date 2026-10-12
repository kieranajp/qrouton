import assert from "node:assert/strict";
import test from "node:test";
import { EDITOR_PRESETS, matchingPreset } from "./presets.js";

test("a preset matches only its exact template", () => {
  assert.deepEqual(
    EDITOR_PRESETS.map((preset) => preset.name),
    ["nvim", "zed", "code", "cursor", "subl"],
  );
  assert.equal(matchingPreset("zed {path}:{line}"), "zed");
  assert.equal(matchingPreset("  code -g {path}:{line} "), "code");
  assert.equal(matchingPreset("code --wait {path}"), undefined);
  assert.equal(matchingPreset(""), undefined);
});
