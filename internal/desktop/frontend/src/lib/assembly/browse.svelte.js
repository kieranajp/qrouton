import { call, Events } from "../wails.js";
import * as go from "./calls.js";
import { filter, repoID } from "./filter.js";
import { pushed } from "./pushed.js";
import { apply, failedOwners, idle } from "./refresh.js";
import {
  counts,
  ordered,
  preselect,
  reconcile,
  roleOf,
  roleOffers,
  rowMeta,
  seed,
  setRole,
  summary,
  upgrading,
} from "./selection.js";
import { refusal } from "./steps.js";

// Search narrows the list; the rows never outgrow the region they scroll in.
const SHOWN = 50;

/**
 * browsing is step 2's state, which the wizard and the picker both draw: the
 * repository list, the owners it is narrowed to, and the roles picked over it.
 * branch is what an editing row joins, which the two arrive at differently.
 * report is where a list that could not be fetched is said out loud, since the
 * two owners each have a footer of their own.
 * @param {() => string} branch
 * @param {(text: string) => void} [report]
 */
export function browsing(branch, report = () => {}) {
  let query = $state("");
  let orgs = $state(/** @type {string[]} */ ([]));
  let owners = $state(/** @type {string[]} */ ([]));
  let refresh = $state(idle());
  let selection = $state(seed());

  call(go.orgs()).then((answer) => {
    if (!answer.ok) return report(refusal(answer.error));
    orgs = answer.value ?? [];
    owners = [...orgs];
  });
  call(go.cached()).then((answer) => {
    if (!answer.ok) return report(refusal(answer.error));
    refresh = idle(answer.value ?? []);
  });

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

  $effect(() =>
    Events.On("orgs:changed", (event) => {
      orgs = event.data ?? [];
      owners = [...orgs];
      refetch();
    }),
  );

  // The events say when a run is live; a run already finished by the time its
  // generation came back must not be reported as one.
  async function refetch() {
    const answer = await call(go.refresh());
    if (!answer.ok) return report(refusal(answer.error));
    const generation = answer.value ?? 0;
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
    /**
     * @param {{id: string, role: 'editing'|'reference'}[]} rows
     * @param {{id: string, role: 'editing'|'reference'}[]} [requested]
     */
    hold: (rows, requested) => (selection = preselect(seed(rows), requested)),
    refetch,
    owner: (org) =>
      (owners = owners.includes(org) ? owners.filter((on) => on !== org) : [...owners, org]),
    role: (id, role) => (selection = setRole(selection, id, role)),
  };
}
