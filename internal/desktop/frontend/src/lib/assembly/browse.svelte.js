import { Events } from "../wails.js";
import * as go from "./calls.js";
import { filter, repoID } from "./filter.js";
import { pushed } from "./pushed.js";
import { apply, failedOwners, idle } from "./refresh.js";
import {
  counts,
  ordered,
  reconcile,
  roleOf,
  roleOffers,
  rowMeta,
  seed,
  setRole,
  summary,
  upgrading,
} from "./selection.js";

// Search narrows the list; the rows never outgrow the region they scroll in.
const SHOWN = 50;

/**
 * browsing is step 2's state, which the wizard and the picker both draw: the
 * repository list, the owners it is narrowed to, and the roles picked over it.
 * branch is what an editing row joins, which the two arrive at differently.
 * @param {() => string} branch
 */
export function browsing(branch) {
  let query = $state("");
  let orgs = $state(/** @type {string[]} */ ([]));
  let owners = $state(/** @type {string[]} */ ([]));
  let refresh = $state(idle());
  let selection = $state(seed());

  go.orgs().then((list) => {
    orgs = list ?? [];
    owners = [...orgs];
  });
  go.cached().then((list) => (refresh = idle(list ?? [])));

  // Pinning the locked rows rather than every picked one: a selection that
  // re-sorted as it was made would move the next row under the pointer.
  let listed = $derived(
    filter({ repos: refresh.repos, owners, query, cap: SHOWN, pinned: selection.locked }),
  );
  let rows = $derived(
    listed.rows.map((row) => ({
      id: row.id,
      meta: rowMeta(selection, row.id, pushed(row.pushed_at)),
      role: roleOf(selection, row.id),
      offers: roleOffers(selection, row.id),
    })),
  );

  $effect(() =>
    Events.On("repos:refresh", (event) => {
      const updated = apply(refresh, event.data ?? {});
      refresh = updated;
      selection = reconcile(selection, updated.repos.map(repoID));
    }),
  );

  // The events say when a run is live; a run already finished by the time its
  // generation came back must not be reported as one.
  async function refetch() {
    const generation = await go.refresh();
    if (generation > refresh.generation) refresh = { ...refresh, generation };
  }

  return {
    get query() {
      return query;
    },
    set query(value) {
      query = value;
    },
    get orgs() {
      return orgs;
    },
    get owners() {
      return owners;
    },
    get failed() {
      return failedOwners(refresh);
    },
    get refreshing() {
      return refresh.active;
    },
    get rows() {
      return rows;
    },
    get shown() {
      return listed.shown;
    },
    get total() {
      return listed.total;
    },
    get tally() {
      return counts(selection);
    },
    get picks() {
      return summary(selection, refresh.repos, branch());
    },
    get ordered() {
      return ordered(selection);
    },
    get upgrading() {
      return upgrading(selection);
    },
    /** @param {{id: string, role: 'editing'|'reference'}[]} rows */
    hold: (rows) => (selection = seed(rows)),
    refetch,
    owner: (org) =>
      (owners = owners.includes(org) ? owners.filter((on) => on !== org) : [...owners, org]),
    role: (id, role) => (selection = setRole(selection, id, role)),
  };
}
