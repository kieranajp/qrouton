import assert from "node:assert/strict";
import test from "node:test";
import { EDITOR_PRESETS, matchingPreset } from "./presets.js";

test("each preset's template runs its own editor on the file and matches back to it", () => {
  for (const preset of EDITOR_PRESETS) {
    assert.equal(matchingPreset(preset.template), preset.name);
    assert.equal(preset.template.split(" ")[0], preset.name);
    assert.ok(preset.template.includes("{path}"), `${preset.name} never opens the file`);
  }
});

test("a preset matches only its exact template", () => {
  assert.equal(matchingPreset("zed {path}:{line}"), "zed");
  assert.equal(matchingPreset("  code -g {path}:{line} "), "code");
  assert.equal(matchingPreset("code --wait {path}"), undefined);
  assert.equal(matchingPreset(""), undefined);
});
