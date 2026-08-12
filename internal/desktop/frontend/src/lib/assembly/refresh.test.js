import assert from "node:assert/strict";
import test from "node:test";
import { apply, failedOwners, idle } from "./refresh.js";

const CACHED = [{ org: "lifesum", name: "api" }];
const FETCHED = [{ org: "lifesum", name: "billing" }];

const live = (generation) => apply(idle(CACHED), { generation, state: "started", owner: "lifesum" });

test("per-owner status tracks started, succeeded and failed independently", () => {
  let refresh = idle();
  refresh = apply(refresh, { generation: 1, state: "started", owner: "lifesum" });
  refresh = apply(refresh, { generation: 1, state: "started", owner: "vimeda" });
  refresh = apply(refresh, { generation: 1, state: "succeeded", owner: "lifesum", repos: FETCHED });
  refresh = apply(refresh, { generation: 1, state: "failed", owner: "vimeda", error: "401" });

  assert.equal(refresh.owners.lifesum.status, "updated");
  assert.deepEqual(refresh.owners.vimeda, { status: "failed", error: "401" });
  assert.deepEqual(failedOwners(refresh), ["vimeda"]);
  assert.equal(refresh.active, true);
});

test("a succeeded owner stops carrying the error its last attempt had", () => {
  let refresh = apply(idle(), { generation: 1, state: "failed", owner: "lifesum", error: "401" });
  refresh = apply(refresh, { generation: 2, state: "succeeded", owner: "lifesum" });
  assert.deepEqual(refresh.owners.lifesum, { status: "updated", error: undefined });
  assert.deepEqual(failedOwners(refresh), []);
});

test("an event from a superseded generation is dropped", () => {
  const refresh = apply(live(2), { generation: 1, state: "succeeded", owner: "lifesum", repos: FETCHED });
  assert.deepEqual(refresh.repos, CACHED);
  assert.equal(refresh.owners.lifesum.status, "fetching");
});

test("a retry carrying a stale generation is dropped", () => {
  let refresh = apply(idle(CACHED), { generation: 1, state: "failed", owner: "vimeda", error: "401" });
  refresh = apply(refresh, { generation: 2, state: "started", owner: "vimeda" });
  refresh = apply(refresh, { generation: 1, state: "succeeded", owner: "vimeda", repos: FETCHED });
  assert.equal(refresh.owners.vimeda.status, "fetching");
  assert.deepEqual(refresh.repos, CACHED);
});

test("complete closes the run out and keeps the owners it reported on", () => {
  let refresh = live(1);
  refresh = apply(refresh, { generation: 1, state: "complete", repos: FETCHED });
  assert.equal(refresh.active, false);
  assert.deepEqual(refresh.repos, FETCHED);
  assert.equal(refresh.owners.lifesum.status, "fetching");
});

test("an event carrying no repositories leaves the ones on screen", () => {
  const refresh = apply(live(1), { generation: 1, state: "succeeded", owner: "lifesum" });
  assert.deepEqual(refresh.repos, CACHED);
});
