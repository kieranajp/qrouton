import assert from "node:assert/strict";
import test from "node:test";
import { fieldError, saveOutcome } from "./errors.js";

test("a field: message error splits into both parts", () => {
  assert.deepEqual(fieldError(new Error("root: cannot be empty")), {
    field: "root",
    message: "cannot be empty",
  });
});

test("a message with no leading field name answers null", () => {
  assert.equal(fieldError(new Error("disk is full")), null);
});

test("a plain string is read the same as an Error", () => {
  assert.deepEqual(fieldError("editor: not installed"), { field: "editor", message: "not installed" });
});

test("a message whose own text carries a colon keeps it intact", () => {
  assert.deepEqual(fieldError(new Error("launch: invalid character '}' looking for: value")), {
    field: "launch",
    message: "invalid character '}' looking for: value",
  });
});

test("a save reporting restartRequired stays open behind the banner", () => {
  assert.deepEqual(saveOutcome({ restartRequired: true }, undefined), {
    close: false,
    restartRequired: true,
    fields: {},
    status: "",
  });
});

test("a save reporting no restart closes exactly as before", () => {
  assert.deepEqual(saveOutcome({ restartRequired: false }, undefined), {
    close: true,
    restartRequired: false,
    fields: {},
    status: "",
  });
});

test("a refused save stays open and names the field without touching the banner", () => {
  const outcome = saveOutcome(undefined, new Error("root: cannot be empty"));
  assert.equal(outcome.close, false);
  assert.equal(outcome.restartRequired, undefined);
  assert.deepEqual(outcome.fields, { root: "cannot be empty" });
  assert.equal(outcome.status, "cannot be empty");
});

test("a refusal with no field name still shows in the footer", () => {
  const outcome = saveOutcome(undefined, new Error("disk is full"));
  assert.deepEqual(outcome.fields, {});
  assert.equal(outcome.status, "disk is full");
});
