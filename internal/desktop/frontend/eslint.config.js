import { loadPolicy } from "./eslint-rules/policy.js";
import commentDiscipline from "./eslint-rules/comment-discipline.js";
import svelteParser from "svelte-eslint-parser";

const policy = loadPolicy();

export default [
  {
    ignores: ["node_modules/**", "src/lib/bridge/generated.js"],
  },
  {
    files: ["**/*.js"],
    languageOptions: {
      ecmaVersion: "latest",
      sourceType: "module",
    },
    plugins: { "comment-discipline": commentDiscipline },
    rules: {
      "comment-discipline/max-comment-run": ["error", { max: policy.maxCommentRun }],
      "comment-discipline/no-narration": ["error", { phrases: policy.narrationPhrases }],
      "comment-discipline/no-path-pointer": ["error", { extensions: policy.pathExtensions }],
    },
  },
  {
    files: ["**/*.svelte"],
    languageOptions: {
      parser: svelteParser,
      parserOptions: { ecmaVersion: "latest", sourceType: "module" },
    },
    plugins: { "comment-discipline": commentDiscipline },
    rules: {
      "comment-discipline/max-comment-run": ["error", { max: policy.maxCommentRun }],
      "comment-discipline/no-narration": ["error", { phrases: policy.narrationPhrases }],
      "comment-discipline/no-path-pointer": ["error", { extensions: policy.pathExtensions }],
    },
  },
];

