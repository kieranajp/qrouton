export const GUIDANCE = "See AGENTS.md: comments default to none, state what IS, and stay one line where earned or two for a real trap.";

export const MACHINE_LINE = /^\s*(?:eslint-(?:disable|enable)(?:\b|$)|eslint-env\b|globals?\s|exported\s|(?:prettier|stylelint)-(?:ignore|disable|enable)\b|(?:istanbul|c8|v8)\s|@(?:ts-|jsx|typedef\b|type\b|param\b|returns?\b|template\b|property\b|arg(?:ument)?\b|return\b|import\b|implements\b|extends\b|satisfies\b)|(?:url|source|component|props)=|svelte-(?:ignore|warning)\b)/i;

export const URL = /https?:\/\/[^\s<>()]+/gi;

/** @param {string} raw */
export const meaningfulLines = (raw) =>
  raw.replace(/^[\s*]+/gm, " ").split("\n").map((line) => line.trim()).filter(Boolean);

/** @param {string[]} lines */
export const machineOnly = (lines) => lines.length > 0 && lines.every((line) => MACHINE_LINE.test(line));

/** @param {string[]} extensions */
export function pathPointer(extensions) {
  const suffix = extensions.map((extension) => extension.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")).join("|");
  return new RegExp(`(?:^|[\\s(\`'\"])(?:[\\w.-]+\\/)+[\\w.-]+\\.(?:${suffix})\\b|\\b[\\w.-]+\\.(?:${suffix}):\\d+\\b`, "i");
}
