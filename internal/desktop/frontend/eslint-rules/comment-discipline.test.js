import test from "node:test";
import assert from "node:assert/strict";
import { Linter, RuleTester } from "eslint";
import svelteParser from "svelte-eslint-parser";
import plugin from "./comment-discipline.js";

const rules = plugin.rules;

test("JavaScript rules cover runs, directives, narration, pointers, and URL spans", () => {
  const ruleTester = new RuleTester({ languageOptions: { ecmaVersion: "latest", sourceType: "module" } });
  ruleTester.run("max-comment-run", rules["max-comment-run"], {
    valid: [
      { code: "// one\n// two\nconst value = 1;", options: [{ max: 2 }] },
      { code: "// one\n// two\n\n// three\n// four\nconst value = 1;", options: [{ max: 2 }] },
      { code: "// one\n// eslint-disable-next-line no-console\n// two\nconst value = 1;", options: [{ max: 1 }] },
      { code: "#!/usr/bin/env node\n// one\nconst value = 1;", options: [{ max: 1 }] },
      { code: "// one\nconst value = 1; // two\n", options: [{ max: 1 }] },
    ],
    invalid: [
      { code: "// one\n// two\n// three\nconst value = 1;", options: [{ max: 2 }], errors: [{ messageId: "tooLong" }] },
      { code: "// one\n// #! not a shebang\n// two\nconst value = 1;", options: [{ max: 1 }], errors: [{ messageId: "tooLong" }] },
      { code: "/**\n * one\n * two\n */\nconst value = 1;", options: [{ max: 2 }], errors: [{ messageId: "tooLong" }] },
    ],
  });

  ruleTester.run("no-narration", rules["no-narration"], {
    valid: [
      { code: "// Fixed for now; the caller still wins.", options: [{ phrases: ["turns out"] }] },
      { code: "// eslint-disable-next-line -- turns out this is needed", options: [{ phrases: ["turns out"] }] },
      { code: "#!/usr/bin/env -S turns out src/tool.js", options: [{ phrases: ["turns out"] }] },
      { code: "/** @type {import(\"./model.js\").Thing} */\nconst value = 1;", options: [{ phrases: ["the problem was"] }] },
    ],
    invalid: [
      { code: "// The double render was a giveaway.", options: [{ phrases: ["was a giveaway"] }], errors: [{ messageId: "narration" }] },
      { code: "// #! turns out this is ordinary prose", options: [{ phrases: ["turns out"] }], errors: [{ messageId: "narration" }] },
      { code: "/** @type {Thing}\n * The problem was a stale ref.\n */", options: [{ phrases: ["the problem was"] }], errors: [{ messageId: "narration" }] },
    ],
  });

  ruleTester.run("no-path-pointer", rules["no-path-pointer"], {
    valid: [
      { code: "// See https://example.com/a/b.js for the upstream issue.", options: [{ extensions: ["js"] }] },
      { code: "// See https://example.com/a/b.js and keep the API stable.", options: [{ extensions: ["js"] }] },
      { code: "#!/usr/bin/env -S node src/tool.js", options: [{ extensions: ["js"] }] },
      { code: "// source=src/components/onboarding/RadioGroup.svelte", options: [{ extensions: ["svelte"] }] },
    ],
    invalid: [
      { code: "// See models/journal-view.js for display shapes.", options: [{ extensions: ["js"] }], errors: [{ messageId: "pointer" }] },
      { code: "// #! src/tool.js is ordinary prose", options: [{ extensions: ["js"] }], errors: [{ messageId: "pointer" }] },
      { code: "// See https://example.com/a/b.js and models/journal-view.js.", options: [{ extensions: ["js"] }], errors: [{ messageId: "pointer" }] },
      { code: "// The guard is at AuthStore.js:142.", options: [{ extensions: ["js"] }], errors: [{ messageId: "pointer" }] },
    ],
  });

  ruleTester.run("no-prose-before-jsdoc", rules["no-prose-before-jsdoc"], {
    valid: [
      { code: "// Explains the surrounding module.\n\n/** @type {number} */\nconst value = 1;" },
      { code: "const prior = 1; // Explains the prior value.\n/** @type {number} */\nconst value = 1;" },
      { code: "// @ts-ignore\n/** @type {number} */\nconst value = 1;" },
      { code: "// Explains the ordinary block.\n/* Ordinary block. */\nconst value = 1;" },
    ],
    invalid: [
      {
        code: "// The selected tab remains visible.\n/** @param {number} selected */\nfunction choose(selected) {}",
        errors: [{ messageId: "adjacent", line: 1, column: 1 }],
      },
    ],
  });
});

function verifySvelte(code, rule, options) {
  const linter = new Linter();
  const setting = options === undefined ? "error" : ["error", options];
  return linter.verify(code, {
    files: ["**/*.svelte"],
    languageOptions: { parser: svelteParser, parserOptions: { ecmaVersion: "latest", sourceType: "module" } },
    plugins: { "comment-discipline": { rules } },
    rules: { [`comment-discipline/${rule}`]: setting },
  }, { filename: "fixture.svelte" });
}

test("Svelte template comments are checked with source locations", () => {
  const long = verifySvelte("<!-- one -->\n<!-- two -->\n<!-- three -->\n<div />", "max-comment-run", { max: 2 });
  assert.equal(long.length, 1);
  assert.equal(long[0].ruleId, "comment-discipline/max-comment-run");
  assert.equal(long[0].line, 1);
  assert.equal(long[0].column, 1);

  const narration = verifySvelte("<main>\n  <!-- The problem was a stale ref. -->\n</main>", "no-narration", { phrases: ["the problem was"] });
  assert.equal(narration.length, 1);
  assert.equal(narration[0].line, 2);
  assert.equal(narration[0].column, 3);
});

test("Svelte template comments nested in block fragments are checked", () => {
  const fixtures = [
    { code: "{#if ready}<!-- The problem was ready. -->{:else}<!-- The problem was waiting. -->{/if}", count: 2 },
    { code: "{#each items as item}<!-- The problem was an item. -->{:else}<!-- The problem was empty. -->{/each}", count: 2 },
    { code: "{#await promise}<!-- The problem was pending. -->{:then value}<!-- The problem was resolved. -->{:catch error}<!-- The problem was rejected. -->{/await}", count: 3 },
    { code: "{#key key}<!-- The problem was a keyed branch. -->{/key}", count: 1 },
    { code: "{#snippet child()}<!-- The problem was a snippet. -->{/snippet}", count: 1 },
  ];
  for (const { code, count } of fixtures) {
    const diagnostics = verifySvelte(code, "no-narration", { phrases: ["the problem was"] });
    assert.equal(diagnostics.length, count, code);
    assert.ok(diagnostics.every((diagnostic) => diagnostic.ruleId === "comment-discipline/no-narration"), code);
  }
});

test("Svelte scripts reject prose immediately before JSDoc", () => {
  const invalid = verifySvelte(`<script>
  // The selected tab remains visible.
  /** @type {number} */
  let selected = 0;
</script>`, "no-prose-before-jsdoc");
  assert.equal(invalid.length, 1);
  assert.equal(invalid[0].ruleId, "comment-discipline/no-prose-before-jsdoc");
  assert.equal(invalid[0].line, 2);
  assert.equal(invalid[0].column, 3);

  const valid = [
    `<script>
  // Explains the surrounding module.

  /** @type {number} */
  let selected = 0;
</script>`,
    `<script>
  let prior = 1; // Explains the prior value.
  /** @type {number} */
  let selected = 0;
</script>`,
    `<script>
  // @ts-ignore
  /** @type {number} */
  let selected = 0;
</script>`,
    `<script>
  // Explains the ordinary block.
  /* Ordinary block. */
  let selected = 0;
</script>`,
  ];
  for (const code of valid) assert.deepEqual(verifySvelte(code, "no-prose-before-jsdoc"), [], code);
});
