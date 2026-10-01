import assert from "node:assert/strict";
import test from "node:test";
import { addRow, fromView, movedPaths, removeRow, toInput } from "./thoughts.js";

const view = {
  default: "",
  derived: "/sessions/thoughts",
  roots: [{ id: "work", path: "/sync/work", orgs: "acme" }],
};
const loaded = { ...fromView(view), derived: view.derived };

test("an untouched form moves nothing", () => {
  assert.equal(movedPaths(loaded, fromView(view)), false);
});

test("a changed default moves sessions already writing to the old one", () => {
  assert.equal(movedPaths(loaded, { ...fromView(view), default: "/elsewhere" }), true);
});

test("typing the derived default out in full moves nothing", () => {
  assert.equal(movedPaths(loaded, { ...fromView(view), default: "/sessions/thoughts/" }), false);
});

test("a changed shared path moves sessions already writing there", () => {
  const form = fromView(view);
  form.roots[0].path = "/sync/elsewhere";
  assert.equal(movedPaths(loaded, form), true);
});

test("a removed shared folder moves sessions already writing there", () => {
  assert.equal(movedPaths(loaded, removeRow(fromView(view), 0)), true);
});

test("an added folder moves nothing", () => {
  const form = addRow(fromView(view));
  form.roots[1] = { id: "club", path: "/sync/club", orgs: "club", saved: false };
  assert.equal(movedPaths(loaded, form), false);
});

test("changed orgs alone move nothing", () => {
  const form = fromView(view);
  form.roots[0].orgs = "acme, acme-labs";
  assert.equal(movedPaths(loaded, form), false);
});

test("the save payload drops which rows were saved", () => {
  assert.deepEqual(toInput(addRow(fromView(view))), {
    default: "",
    roots: [
      { id: "work", path: "/sync/work", orgs: "acme" },
      { id: "", path: "", orgs: "" },
    ],
  });
});
