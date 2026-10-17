import fs from "node:fs";
import path from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";
import postcss from "postcss";
import svelteParser from "svelte-eslint-parser";
import { GUIDANCE, MACHINE_LINE, URL, machineOnly, meaningfulLines, pathPointer } from "../eslint-rules/comment-shape.js";
import { loadPolicy } from "../eslint-rules/policy.js";

const frontendRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const rule = "comment-discipline";

function normalize(comment) {
  return meaningfulLines(comment.text ?? comment.textValue ?? comment.data ?? "");
}

function isDirective(comment) {
  return machineOnly(normalize(comment));
}

function prose(comment) {
  return normalize(comment).filter((line) => !MACHINE_LINE.test(line)).join(" ");
}

function sourceLines(source) {
  return source.split("\n");
}

function ownsLine(source, comment) {
  const lines = sourceLines(source);
  return lines[comment.start.line - 1].slice(0, comment.start.column - 1).trim() === "";
}

function diagnostic(file, comment, ruleName, message) {
  return `${file}:${comment.start.line}:${comment.start.column}: ${rule}/${ruleName}: ${message}`;
}

function checkAst(file, source, root, policy) {
  const comments = [];
  root.walkComments((node) => comments.push({
    text: node.text,
    start: node.source.start,
    end: node.source.end,
  }));
  comments.sort((left, right) => left.start.offset - right.start.offset);
  const pointer = pathPointer(policy.pathExtensions);
  const diagnostics = [];
  let run = [];
  const flush = () => {
    if (run.length === 0) return;
    const height = run.reduce((sum, comment) => sum + comment.end.line - comment.start.line + 1, 0);
    if (height > policy.maxCommentRun) {
      diagnostics.push(diagnostic(file, run[0], "max-comment-run", `comment runs ${height} lines; the cap is ${policy.maxCommentRun}. Say the one thing code cannot, or delete it. ${GUIDANCE}`));
    }
    run = [];
  };
  for (const comment of comments) {
    if (!isDirective(comment)) {
      const body = prose(comment).toLowerCase();
      const phrase = policy.narrationPhrases.find((candidate) => body.includes(candidate));
      if (phrase) diagnostics.push(diagnostic(file, comment, "no-narration", `comment narration contains ${JSON.stringify(phrase)}. State what IS, not how the code got here. ${GUIDANCE}`));
      if (pointer.test(prose(comment).replace(URL, ""))) {
        diagnostics.push(diagnostic(file, comment, "no-path-pointer", `file and line pointers go stale. Name the subject, not its location. ${GUIDANCE}`));
      }
    }
    if (isDirective(comment) || !ownsLine(source, comment)) {
      flush();
      continue;
    }
    const previous = run.at(-1);
    if (previous && comment.start.line !== previous.end.line + 1) flush();
    run.push(comment);
  }
  flush();
  return diagnostics;
}

function checkCss(file, source, policy) {
  let root;
  try {
    root = postcss.parse(source, { from: file });
  } catch (error) {
    throw new Error(`${file}:${error.line ?? 1}:${error.column ?? 1}: ${rule}/parse-error: ${error.reason ?? error.message}`, { cause: error });
  }
  return checkAst(file, source, root, policy);
}

function checkSvelte(file, source, policy) {
  const parsed = svelteParser.parseForESLint(source, { filePath: file });
  const style = parsed.services.getStyleContext();
  if (style.status === "no-style-element" || style.status === "unknown-lang") return [];
  if (style.status === "parse-error") {
    throw new Error(`${file}:${style.error.lineNumber ?? 1}:${style.error.column ?? 1}: ${rule}/parse-error: ${style.error.message}`, { cause: style.error });
  }
  return checkAst(file, source, style.sourceAst, policy);
}

function filesUnder(directory) {
  const result = [];
  const entries = fs.readdirSync(directory, { withFileTypes: true }).sort((left, right) => left.name.localeCompare(right.name));
  for (const entry of entries) {
    if (entry.name === "node_modules") continue;
    const absolute = path.join(directory, entry.name);
    if (entry.isDirectory()) result.push(...filesUnder(absolute));
    else if (entry.isFile() && (entry.name.endsWith(".css") || entry.name.endsWith(".svelte"))) result.push(absolute);
  }
  return result;
}

export function checkTree(root = frontendRoot, policy = loadPolicy()) {
  const diagnostics = [];
  for (const absolute of filesUnder(root)) {
    const source = fs.readFileSync(absolute, "utf8");
    const relative = path.relative(root, absolute).split(path.sep).join("/");
    diagnostics.push(...(absolute.endsWith(".css") ? checkCss(relative, source, policy) : checkSvelte(relative, source, policy)));
  }
  return diagnostics.sort();
}

if (process.argv[1] && import.meta.url === pathToFileURL(path.resolve(process.argv[1])).href) {
  try {
    const diagnostics = checkTree();
    if (diagnostics.length > 0) {
      console.error(diagnostics.join("\n"));
      process.exitCode = 1;
    }
  } catch (error) {
    console.error(error.message);
    process.exitCode = 1;
  }
}
