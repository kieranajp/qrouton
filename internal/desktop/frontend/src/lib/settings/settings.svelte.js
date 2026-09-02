import { call } from "../wails.js";
import * as go from "./calls.js";
import { loadFailure, saveOutcome } from "./errors.js";
import { addOrg, removeOrg } from "./orgs.js";

// Restart-required saves keep the panel open behind the banner.
/** @param {() => void} onClose */
export function settings(onClose) {
  const form = $state({
    orgs: /** @type {string[]} */ ([]),
    root: "",
    editor: "",
    launch: "",
    linear: "",
    linearPath: "",
    stickerLabels: {
      star: "",
      bookmark: "",
      question: "",
      exclamation: "",
    },
  });
  let orgInput = $state("");
  let fields = $state(/** @type {Record<string, string>} */ ({}));
  let status = $state("");
  let saving = $state(false);
  let restartRequired = $state(false);

  call(go.load()).then((answer) => {
    if (!answer.ok) {
      status = loadFailure(answer.error);
      return;
    }
    const loaded = answer.value;
    form.orgs = loaded?.orgs ?? [];
    form.root = loaded?.root ?? "";
    form.editor = loaded?.editor ?? "";
    form.launch = loaded?.launch ?? "";
    form.linear = loaded?.linear ?? "";
    form.linearPath = loaded?.linearPath ?? "";
    form.stickerLabels = {
      star: loaded?.stickerLabels?.star ?? "",
      bookmark: loaded?.stickerLabels?.bookmark ?? "",
      question: loaded?.stickerLabels?.question ?? "",
      exclamation: loaded?.stickerLabels?.exclamation ?? "",
    };
    if (loaded?.linearError) fields = { ...fields, linear: loaded.linearError };
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
        linear: form.linear,
        stickerLabels: { ...form.stickerLabels },
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
