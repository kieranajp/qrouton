import assert from "node:assert/strict";
import test from "node:test";
import { fill } from "./ticket.js";

const URL = "https://linear.app/lifesum/issue/LIF-2841";
const LOADED = { url: URL, title: "Extract billing service", body: "Pull invoicing out." };

test("a result for the current URL fills the fields that are empty", () => {
  assert.deepEqual(fill({ name: "", description: "", ticket: URL }, LOADED), {
    name: "Extract billing service",
    description: "Pull invoicing out.",
  });
});

test("a result for a URL the field has since moved off is discarded", () => {
  const draft = { name: "", description: "", ticket: URL + "-renamed" };
  assert.deepEqual(fill(draft, LOADED), { name: "", description: "" });
});

test("typed text is never overwritten, field by field", () => {
  const draft = { name: "Billing split", description: "", ticket: URL };
  assert.deepEqual(fill(draft, LOADED), {
    name: "Billing split",
    description: "Pull invoicing out.",
  });
});

test("a field holding only spaces counts as empty", () => {
  const draft = { name: "   ", description: "  ", ticket: URL };
  assert.deepEqual(fill(draft, LOADED), {
    name: "Extract billing service",
    description: "Pull invoicing out.",
  });
});

// Fetching trims the URL, so the field it came back to often has not.
test("stray whitespace around the URL does not discard its own result", () => {
  const draft = { name: "", description: "", ticket: " " + URL + " " };
  assert.equal(fill(draft, LOADED).name, "Extract billing service");
});

test("a result carrying no URL fills nothing", () => {
  const draft = { name: "", description: "", ticket: "" };
  assert.deepEqual(fill(draft, { title: "Stray", body: "Stray" }), { name: "", description: "" });
});
