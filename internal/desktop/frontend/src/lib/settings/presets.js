export const EDITOR_PRESETS = Object.freeze([
  Object.freeze({ name: "nvim", template: "nvim +{line} {path}" }),
  Object.freeze({ name: "zed", template: "zed {path}:{line}" }),
  Object.freeze({ name: "code", template: "code -g {path}:{line}" }),
  Object.freeze({ name: "cursor", template: "cursor -g {path}:{line}" }),
  Object.freeze({ name: "subl", template: "subl {path}:{line}" }),
]);

/** @param {string} editor */
export const matchingPreset = (editor) => EDITOR_PRESETS.find((preset) => preset.template === editor.trim())?.name;
