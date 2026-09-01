import assert from "node:assert/strict";
import test from "node:test";
import { applies, claimSeed, fill, loader } from "./ticket.js";

const URL = "https://linear.app/lifesum/issue/LIF-2841";
const LOADED = {
  url: URL,
  title: "Extract billing service",
  body: "Pull invoicing out.",
  branchDescription: "extract-billing-service",
};

test("a result for the current URL fills the fields that are empty", () => {
  assert.deepEqual(fill({ name: "", description: "", ticket: URL }, LOADED), {
    name: "Extract billing service",
    branchDescription: "extract-billing-service",
    description: "Pull invoicing out.",
  });
});

test("a result for a URL the field has since moved off is discarded", () => {
  const draft = { name: "", description: "", ticket: URL + "-renamed" };
  assert.deepEqual(fill(draft, LOADED), { name: "", branchDescription: "", description: "" });
  assert.equal(applies(draft, LOADED), false);
});

test("typed text is never overwritten, field by field", () => {
  const draft = {
    name: "Billing split",
    branchDescription: "billing-split",
    description: "",
    ticket: URL,
  };
  assert.deepEqual(fill(draft, LOADED), {
    name: "Billing split",
    branchDescription: "billing-split",
    description: "Pull invoicing out.",
  });
});

test("a field holding only spaces counts as empty", () => {
  const draft = { name: "   ", description: "  ", ticket: URL };
  assert.deepEqual(fill(draft, LOADED), {
    name: "Extract billing service",
    branchDescription: "extract-billing-service",
    description: "Pull invoicing out.",
  });
});

// Fetching trims the URL, so the field it came back to often has not.
test("stray whitespace around the URL does not discard its own result", () => {
  const draft = { name: "", description: "", ticket: " " + URL + " " };
  assert.equal(fill(draft, LOADED).name, "Extract billing service");
  assert.equal(applies(draft, LOADED), true);
});

test("a result carrying no URL fills nothing", () => {
  const draft = { name: "", description: "", ticket: "" };
  assert.deepEqual(fill(draft, { title: "Stray", body: "Stray" }), {
    name: "",
    branchDescription: "",
    description: "",
  });
});

test("one external seed fetches once and a failed fetch can be retried manually", async () => {
  const draft = { name: "", description: "", ticket: "" };
  let attempts = 0;
  let failures = 0;
  const fetching = [];
  const tickets = loader(
    draft,
    async (url) => {
      attempts++;
      if (attempts === 1) throw new Error("Linear unavailable");
      return { ...LOADED, url };
    },
    {
      fetching: (active) => fetching.push(active),
      loaded: (fields) => Object.assign(draft, fields),
      failed: () => failures++,
    },
  );

  const automatic = tickets.seed(URL);
  assert.equal(tickets.seed(URL), false);
  assert.equal(tickets.seed(URL + "-other"), false);
  assert.equal(await automatic, false);
  assert.equal(attempts, 1);
  assert.equal(failures, 1);
  assert.deepEqual(fetching, [true, false]);

  assert.equal(await tickets.load(), true);
  assert.equal(attempts, 2);
  assert.equal(failures, 1);
  assert.deepEqual(fetching, [true, false, true, false]);
  assert.equal(draft.name, LOADED.title);
  assert.equal(draft.branchDescription, LOADED.branchDescription);
  assert.equal(draft.description, LOADED.body);
  assert.equal(claimSeed("manual", URL), "");
});
