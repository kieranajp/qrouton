import * as settings from "../settings/calls.js";
import { addOrg, removeOrg } from "../settings/orgs.js";
import * as go from "./calls.js";
import { firstRunOutcome } from "./outcome.js";
import { last } from "./screens.js";

/**
 * firstRun is the five screens as one step machine. Both answers stay in frontend
 * state until the final button, so quitting partway leaves nothing
 * half-configured. A refusal keeps the user on the last screen with both answers
 * in hand and the gate still up; a success leaves the flow busy and waits for Go,
 * which either drops the gate or replaces the window.
 */
export function firstRun() {
  const form = $state({ orgs: /** @type {string[]} */ ([]), root: "" });
  let step = $state(0);
  let orgInput = $state("");
  let fields = $state(/** @type {Record<string, string>} */ ({}));
  let status = $state("");
  let busy = $state(false);

  // The prefill is Settings' own Load: the same config, so asking twice would be
  // two owners of one fact.
  settings.load().then((loaded) => {
    form.orgs = loaded?.orgs ?? [];
    form.root = loaded?.root ?? "";
  });

  function add() {
    form.orgs = addOrg(form.orgs, orgInput);
    orgInput = "";
  }

  function remove(org) {
    form.orgs = removeOrg(form.orgs, org);
  }

  function back() {
    if (step > 0) step--;
  }

  function next() {
    if (step < last) {
      step++;
      return;
    }
    save();
  }

  async function save() {
    if (busy) return;
    busy = true;
    let result, err;
    try {
      result = await go.save({ orgs: form.orgs, root: form.root });
    } catch (thrown) {
      err = thrown;
    }

    const outcome = firstRunOutcome(result, err);
    fields = outcome.fields;
    status = outcome.status;
    // Only a refusal hands the button back; a success is waiting for Go.
    if (err) busy = false;
  }

  return {
    form,
    get step() {
      return step;
    },
    get orgInput() {
      return orgInput;
    },
    set orgInput(value) {
      orgInput = value;
    },
    get fields() {
      return fields;
    },
    get status() {
      return status;
    },
    get busy() {
      return busy;
    },
    add,
    remove,
    back,
    next,
  };
}
