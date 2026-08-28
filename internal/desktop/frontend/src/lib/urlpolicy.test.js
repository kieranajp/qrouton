import assert from "node:assert/strict";
import test from "node:test";
import { isAllowedURL } from "./urlpolicy.js";

test("http and https are allowed", () => {
  assert.equal(isAllowedURL("http://example.com"), true);
  assert.equal(isAllowedURL("https://example.com/doc"), true);
});

test("javascript, file, and data schemes are refused", () => {
  assert.equal(isAllowedURL("javascript:alert(1)"), false);
  assert.equal(isAllowedURL("file:///etc/passwd"), false);
  assert.equal(isAllowedURL("data:text/html,<script>alert(1)</script>"), false);
});

test("a relative path has no scheme to trust", () => {
  assert.equal(isAllowedURL("notes/child.md"), false);
});
