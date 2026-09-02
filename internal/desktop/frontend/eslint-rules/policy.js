import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const policyPath = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "../../../../comment-discipline.json");

export function loadPolicy(file = policyPath) {
  let value;
  try {
    value = JSON.parse(fs.readFileSync(file, "utf8"));
  } catch (error) {
    throw new Error(`read policy: ${error.message}`, { cause: error });
  }

  if (!value || typeof value !== "object" || Array.isArray(value)) {
    throw new Error("policy must be a JSON object");
  }
  const keys = Object.keys(value).sort();
  const expected = ["maxCommentRun", "narrationPhrases", "pathExtensions", "schemaVersion"];
  if (keys.length !== expected.length || keys.some((key, index) => key !== expected[index])) {
    throw new Error(`policy must contain exactly: ${expected.join(", ")}`);
  }
  if (value.schemaVersion !== 1) throw new Error("policy schemaVersion must be 1");
  if (!Number.isInteger(value.maxCommentRun) || value.maxCommentRun < 1) {
    throw new Error("policy maxCommentRun must be a positive integer");
  }
  validateList(value.narrationPhrases, "narrationPhrases");
  validateList(value.pathExtensions, "pathExtensions", true);
  return value;
}

function validateList(values, name, extension = false) {
  if (!Array.isArray(values) || values.length === 0) throw new Error(`policy ${name} must not be empty`);
  const seen = new Set();
  for (const value of values) {
    if (typeof value !== "string" || value === "" || value.trim() !== value || value !== value.toLowerCase()) {
      throw new Error(`policy ${name} entries must be non-empty, trimmed, and lowercase`);
    }
    if (extension && (value.startsWith(".") || value.includes("/") || value.includes("\\"))) {
      throw new Error("policy pathExtensions entries must omit dots and path separators");
    }
    if (seen.has(value)) throw new Error(`policy ${name} contains duplicate ${JSON.stringify(value)}`);
    seen.add(value);
  }
}

