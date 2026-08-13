import assert from "node:assert/strict";
import test from "node:test";
import { addOrg, removeOrg } from "./orgs.js";

test("adding trims the entry and appends it", () => {
  assert.deepEqual(addOrg(["acme"], "  second-org  "), ["acme", "second-org"]);
});

test("adding a blank entry leaves the list alone", () => {
  assert.deepEqual(addOrg(["acme"], "   "), ["acme"]);
});

test("adding a duplicate entry leaves the list alone", () => {
  assert.deepEqual(addOrg(["acme", "second-org"], "acme"), ["acme", "second-org"]);
});

test("removing drops exactly the named org and keeps the rest in order", () => {
  assert.deepEqual(removeOrg(["acme", "second-org", "third"], "second-org"), ["acme", "third"]);
});

test("removing an org not in the list leaves it unchanged", () => {
  assert.deepEqual(removeOrg(["acme"], "second-org"), ["acme"]);
});
