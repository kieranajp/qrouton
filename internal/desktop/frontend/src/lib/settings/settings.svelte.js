import { SETTINGS_FIELDS, UI_SCALE_EVENT } from "../bridge/generated.js";
import { call, Events } from "../wails.js";
import * as go from "./calls.js";
import { loadFailure, saveOutcome } from "./errors.js";
import { addOrg, removeOrg } from "./orgs.js";
import { addRow, fromView, removeRow, toInput } from "./thoughts.js";

/** Restart-required saves keep the panel open behind the banner.
 * @param {() => void} onClose */
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
    chime: true,
    uiScale: 100,
    uiScaleSteps: /** @type {number[]} */ ([]),
    thoughts: fromView(undefined),
  });
  let loadedThoughts = $state({ ...fromView(undefined), derived: "" });
  let orgInput = $state("");
  let fields = $state(/** @type {Partial<Record<import("../bridge/generated.js").SettingsField, string>>} */ ({}));
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
    form.chime = loaded?.chime ?? true;
    form.uiScale = loaded?.uiScale ?? 100;
    form.uiScaleSteps = loaded?.uiScaleSteps ?? [];
    form.thoughts = fromView(loaded?.thoughts);
    loadedThoughts = { ...fromView(loaded?.thoughts), derived: loaded?.thoughts?.derived ?? "" };
    if (loaded?.linearError) fields = { ...fields, [SETTINGS_FIELDS.LINEAR]: loaded.linearError };
  });

  $effect(() =>
    Events.On(UI_SCALE_EVENT, (event) => {
      if (typeof event.data === "number") form.uiScale = event.data;
    }),
  );

  function add() {
    form.orgs = addOrg(form.orgs, orgInput);
    orgInput = "";
  }

  function remove(org) {
    form.orgs = removeOrg(form.orgs, org);
  }

  function addFolder() {
    form.thoughts = addRow(form.thoughts);
  }

  /** @param {number} index */
  function removeFolder(index) {
    form.thoughts = removeRow(form.thoughts, index);
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
        chime: form.chime,
        uiScale: form.uiScale,
        thoughts: toInput(form.thoughts),
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
    get loadedThoughts() {
      return loadedThoughts;
    },
    add,
    remove,
    addFolder,
    removeFolder,
    save,
    cancel,
    quitAndRelaunch,
  };
}
