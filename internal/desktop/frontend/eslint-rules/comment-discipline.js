const GUIDANCE = "See AGENTS.md: comments default to none, state what IS, and stay one line where earned or two for a real trap.";

const MACHINE_LINE = /^\s*(?:eslint-(?:disable|enable)(?:\b|$)|eslint-env\b|globals?\s|exported\s|(?:prettier|stylelint)-(?:ignore|disable|enable)\b|(?:istanbul|c8|v8)\s|@(?:ts-|jsx|typedef\b|type\b|param\b|returns?\b|template\b|property\b|arg(?:ument)?\b|return\b|import\b|implements\b|extends\b|satisfies\b)|#!|(?:url|source|component|props)=|svelte-(?:ignore|warning)\b)/i;
const URL = /https?:\/\/[^\s<>()]+/gi;

const text = (comment) => {
  const value = comment.value ?? comment.data ?? "";
  return value.replace(/^[\s*]+/gm, " ");
};

const lines = (comment) => text(comment).split("\n").map((line) => line.trim()).filter(Boolean);

const isDirective = (comment) => {
  if (comment.type === "Shebang") return true;
  const meaningful = lines(comment);
  return meaningful.length > 0 && meaningful.every((line) => MACHINE_LINE.test(line));
};

const prose = (comment) => lines(comment).filter((line) => !MACHINE_LINE.test(line)).join(" ");

const height = (comment) => comment.loc.end.line - comment.loc.start.line + 1;

const ownsItsLine = (sourceCode, comment) =>
  sourceCode.lines[comment.loc.start.line - 1].slice(0, comment.loc.start.column).trim() === "";

function pathPointer(extensions) {
  const suffix = extensions.map((extension) => extension.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")).join("|");
  return new RegExp(`(?:^|[\\s(\`'\"])(?:[\\w.-]+\\/)+[\\w.-]+\\.(?:${suffix})\\b|\\b[\\w.-]+\\.(?:${suffix}):\\d+\\b`, "i");
}

function commentsFor(sourceCode) {
  const comments = [...sourceCode.getAllComments()];
  const services = sourceCode.parserServices;
  const html = services?.getSvelteHtmlAst?.();
  if (!html) return comments;

  const seen = new WeakSet();
  const visit = (node) => {
    if (!node || typeof node !== "object" || seen.has(node)) return;
    seen.add(node);
    if (node.type === "Comment") {
      const start = node.start;
      const end = node.end;
      comments.push({
        type: "HTML",
        value: node.data,
        range: [start, end],
        loc: { start: sourceCode.getLocFromIndex(start), end: sourceCode.getLocFromIndex(end) },
      });
    }
    for (const value of Object.values(node)) {
      if (Array.isArray(value)) {
        for (const child of value) visit(child);
      } else {
        visit(value);
      }
    }
  };
  visit(html);
  return comments.sort((left, right) => left.range[0] - right.range[0]);
}

function report(context, comment, messageId, data) {
  context.report({ loc: comment.loc, messageId, data });
}

const maxCommentRun = {
  meta: {
    type: "suggestion",
    docs: { description: "Cap consecutive standalone comment lines." },
    schema: [{ type: "object", properties: { max: { type: "integer", minimum: 1 } }, additionalProperties: false }],
    messages: {
      tooLong: "This comment runs {{lines}} lines; the cap is {{max}}. Say the one thing that is not already in the code, or delete it. " + GUIDANCE,
    },
  },
  create(context) {
    const max = context.options[0]?.max;
    if (!Number.isInteger(max) || max < 1) throw new Error("comment-discipline max must be a positive integer");
    const sourceCode = context.sourceCode ?? context.getSourceCode();
    return {
      Program() {
        let run = [];
        const flush = () => {
          const count = run.reduce((total, comment) => total + height(comment), 0);
          if (count > max) report(context, { ...run[0], loc: { start: run[0].loc.start, end: run.at(-1).loc.end } }, "tooLong", { lines: count, max });
          run = [];
        };
        for (const comment of commentsFor(sourceCode)) {
          if (isDirective(comment) || !ownsItsLine(sourceCode, comment)) {
            flush();
            continue;
          }
          const previous = run.at(-1);
          if (previous && comment.loc.start.line !== previous.loc.end.line + 1) flush();
          run.push(comment);
        }
        flush();
      },
    };
  },
};

const noNarration = {
  meta: {
    type: "suggestion",
    docs: { description: "Reject comments that narrate the debugging journey." },
    schema: [{ type: "object", properties: { phrases: { type: "array", items: { type: "string" } } }, additionalProperties: false }],
    messages: { narration: 'A comment states what IS, not how the code got here. "{{phrase}}" describes the journey. ' + GUIDANCE },
  },
  create(context) {
    const phrases = context.options[0]?.phrases;
    if (!Array.isArray(phrases) || phrases.length === 0) throw new Error("comment-discipline phrases must not be empty");
    const sourceCode = context.sourceCode ?? context.getSourceCode();
    return { Program() {
      for (const comment of commentsFor(sourceCode)) {
        if (isDirective(comment)) continue;
        const body = prose(comment).toLowerCase();
        const phrase = phrases.find((candidate) => body.includes(candidate));
        if (phrase) report(context, comment, "narration", { phrase });
      }
    } };
  },
};

const noPathPointer = {
  meta: {
    type: "suggestion",
    docs: { description: "Reject file and line pointers in comments." },
    schema: [{ type: "object", properties: { extensions: { type: "array", items: { type: "string" } } }, additionalProperties: false }],
    messages: { pointer: "A file or line pointer in a comment goes stale. Name the thing, not where it lives. " + GUIDANCE },
  },
  create(context) {
    const extensions = context.options[0]?.extensions;
    if (!Array.isArray(extensions) || extensions.length === 0) throw new Error("comment-discipline extensions must not be empty");
    const pointer = pathPointer(extensions);
    const sourceCode = context.sourceCode ?? context.getSourceCode();
    return { Program() {
      for (const comment of commentsFor(sourceCode)) {
        if (isDirective(comment)) continue;
        const body = prose(comment).replace(URL, "");
        if (pointer.test(body)) report(context, comment, "pointer");
      }
    } };
  },
};

export default {
  rules: {
    "max-comment-run": maxCommentRun,
    "no-narration": noNarration,
    "no-path-pointer": noPathPointer,
  },
};

export { isDirective, pathPointer };
