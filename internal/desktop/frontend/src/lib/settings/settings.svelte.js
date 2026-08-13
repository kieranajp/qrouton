import * as go from "./calls.js";
import { saveOutcome } from "./errors.js";
import { addOrg, removeOrg } from "./orgs.js";

/**
 * settings is the panel's one screen: config.Config's four fields, loaded on
 * construction. onClose is called once a save needs nothing further from the
 * user — a save asking for a restart stays open behind a banner instead.
 * @param {() => void} onClose
 */
export function settings(onClose) {
  const form = $state({ orgs: /** @type {string[]} */ ([]), root: "", editor: "", launch: "" });
  let orgInput = $state("");
  let fields = $state(/** @type {Record<string, string>} */ ({}));
  let status = $state("");
  let saving = $state(false);
  let restartRequired = $state(false);

  go.load().then((loaded) => {
    form.orgs = loaded?.orgs ?? [];
    form.root = loaded?.root ?? "";
    form.editor = loaded?.editor ?? "";
    form.launch = loaded?.launch ?? "";
  });

  function add() {
    form.orgs = addOrg(form.orgs, orgInput);
    orgInput = "";
  }

  function remove(org) {
    form.orgs = removeOrg(form.orgs, org);
  }

  async function save() {
    if (saving) return;
    saving = true;
    let result, err;
    try {
      result = await go.save({
        orgs: form.orgs,
        root: form.root,
        editor: form.editor,
        launch: form.launch,
      });
    } catch (thrown) {
      err = thrown;
    }
    saving = false;

    const outcome = saveOutcome(result, err);
    fields = outcome.fields;
    status = outcome.status;
    if (outcome.restartRequired !== undefined) restartRequired = outcome.restartRequired;
    if (outcome.close) onClose();
  }

  function cancel() {
    onClose();
  }

  // Quitting ends the process; there is nothing here to await.
  function quitAndRelaunch() {
    go.quit();
  }

  return {
    form,
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
    get saving() {
      return saving;
    },
    get restartRequired() {
      return restartRequired;
    },
    add,
    remove,
    save,
    cancel,
    quitAndRelaunch,
  };
}
