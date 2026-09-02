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
      { code: "// one\nconst value = 1; // two\n", options: [{ max: 1 }] },
    ],
    invalid: [
      { code: "// one\n// two\n// three\nconst value = 1;", options: [{ max: 2 }], errors: [{ messageId: "tooLong" }] },
      { code: "/**\n * one\n * two\n */\nconst value = 1;", options: [{ max: 2 }], errors: [{ messageId: "tooLong" }] },
    ],
  });

  ruleTester.run("no-narration", rules["no-narration"], {
    valid: [
      { code: "// Fixed for now; the caller still wins.", options: [{ phrases: ["turns out"] }] },
      { code: "// eslint-disable-next-line -- turns out this is needed", options: [{ phrases: ["turns out"] }] },
      { code: "/** @type {import(\"./model.js\").Thing} */\nconst value = 1;", options: [{ phrases: ["the problem was"] }] },
    ],
    invalid: [
      { code: "// The double render was a giveaway.", options: [{ phrases: ["was a giveaway"] }], errors: [{ messageId: "narration" }] },
      { code: "/** @type {Thing}\n * The problem was a stale ref.\n */", options: [{ phrases: ["the problem was"] }], errors: [{ messageId: "narration" }] },
    ],
  });

  ruleTester.run("no-path-pointer", rules["no-path-pointer"], {
    valid: [
      { code: "// See https://example.com/a/b.js for the upstream issue.", options: [{ extensions: ["js"] }] },
      { code: "// See https://example.com/a/b.js and keep the API stable.", options: [{ extensions: ["js"] }] },
      { code: "// source=src/components/onboarding/RadioGroup.svelte", options: [{ extensions: ["svelte"] }] },
    ],
    invalid: [
      { code: "// See models/journal-view.js for display shapes.", options: [{ extensions: ["js"] }], errors: [{ messageId: "pointer" }] },
      { code: "// See https://example.com/a/b.js and models/journal-view.js.", options: [{ extensions: ["js"] }], errors: [{ messageId: "pointer" }] },
      { code: "// The guard is at AuthStore.js:142.", options: [{ extensions: ["js"] }], errors: [{ messageId: "pointer" }] },
    ],
  });
});

function verifySvelte(code, rule, options) {
  const linter = new Linter();
  return linter.verify(code, {
    files: ["**/*.svelte"],
    languageOptions: { parser: svelteParser, parserOptions: { ecmaVersion: "latest", sourceType: "module" } },
    plugins: { "comment-discipline": { rules } },
    rules: { [`comment-discipline/${rule}`]: ["error", options] },
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
