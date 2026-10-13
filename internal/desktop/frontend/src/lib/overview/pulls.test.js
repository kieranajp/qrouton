import assert from "node:assert/strict";
import test from "node:test";
import { SIGNED_OUT, pullRows } from "./pulls.js";

const pr = (repo, number, title) => ({ repo, number, title, url: `https://github.com/${repo}/pull/${number}` });

test("pull requests become rows in the order they arrive", () => {
  const shown = pullRows({ items: [pr("acme/web", 12, "Add pulls"), pr("acme/api", 3, "Serve pulls")], signedOut: false });
  assert.deepEqual(shown, {
    rows: [
      { key: "acme/web#12", title: "Add pulls", number: "#12", url: "https://github.com/acme/web/pull/12" },
      { key: "acme/api#3", title: "Serve pulls", number: "#3", url: "https://github.com/acme/api/pull/3" },
    ],
    note: "",
  });
});

test("a missing or rejected credential is one line in place of the rows", () => {
  assert.deepEqual(pullRows({ items: [pr("acme/web", 12, "Add pulls")], signedOut: true }), { rows: [], note: SIGNED_OUT });
});

test("no pull requests draws nothing", () => {
  assert.equal(pullRows({ items: [], signedOut: false }), null);
  assert.equal(pullRows(undefined), null);
});
