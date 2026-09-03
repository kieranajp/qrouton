import * as settings from "../settings/calls.js";
import { loadFailure } from "../settings/errors.js";
import { addOrg, removeOrg } from "../settings/orgs.js";
import { call } from "../wails.js";
import * as go from "./calls.js";
import { firstRunOutcome } from "./outcome.js";
import { last } from "./screens.js";

// First-run answers remain provisional until the final screen succeeds.
export function firstRun() {
  const form = $state({ orgs: /** @type {string[]} */ ([]), root: "" });
  let step = $state(0);
  let orgInput = $state("");
  let fields = $state(/** @type {Record<string, string>} */ ({}));
  let status = $state("");
  let busy = $state(false);
  let login = $state("");

  // The prefill is Settings' own Load: the same config, so asking twice would be
  // two owners of one fact.
  call(settings.load()).then((answer) => {
    if (!answer.ok) {
      status = loadFailure(answer.error);
      return;
    }
    form.orgs = answer.value?.orgs ?? [];
    form.root = answer.value?.root ?? "";
  });

  // Resolved off the render: shelling out to gh costs more than the screen does,
  // and an account it cannot name is one the screen offers to sign in.
  call(go.login()).then((answer) => (login = answer.ok ? (answer.value ?? "") : ""));

  function add() {
    form.orgs = addOrg(form.orgs, orgInput);
    orgInput = "";
  }

  function remove(org) {
    form.orgs = removeOrg(form.orgs, org);
  }

  // A cancelled picker answers "" with no error, which leaves the field as the
  // user had it; anything thrown is a real failure and would otherwise look like
  // a button that does nothing.
  async function choose() {
    try {
      const chosen = await go.chooseRoot();
      if (chosen) form.root = chosen;
    } catch (err) {
      status = String(err?.message ?? err ?? "");
    }
  }

  function back() {
    if (step > 0) step--;
  }

  // A typed owner is committed on the way out. Chips are the only way to answer
  // screen 4, and there is no Add button beside the field to press instead.
  function next() {
    add();
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
    get login() {
      return login;
    },
    add,
    remove,
    choose,
    back,
    next,
  };
}
