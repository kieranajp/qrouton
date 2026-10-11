import { UI_SCALE_EVENT } from "../bridge/generated.js";
import { call, Events } from "../wails.js";
import * as go from "./calls.js";
import { loadFailure, saveOutcome } from "./errors.js";
import { toInput as stickersInput } from "./stickers.js";
import { firstErroredTab } from "./tabs.js";
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
    stickers: /** @type {import("../session/stickers.js").Sticker[]} */ ([]),
    chime: true,
    uiScale: 100,
    uiScaleSteps: /** @type {number[]} */ ([]),
    thoughts: fromView(undefined),
    pinned: { root: false, orgs: false },
    runners: /** @type {{id: string, label: string}[]} */ ([]),
  });
  let tab = $state("general");
  let linear = $state(/** @type {import("./calls.js").LinearView} */ ({ path: "", snippet: "", replaces: false }));
  let installed = $state(false);
  let installing = $state(false);
  let actionError = $state("");
  let github = $state(/** @type {{state: string, login: string, variable: string} | null} */ (null));
  let checkingGitHub = $state(false);
  let loadedThoughts = $state({ ...fromView(undefined), derived: "" });
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
    linear = loaded?.linear ?? linear;
    form.stickers = (loaded?.stickers ?? []).map((item) => ({ ...item }));
    form.chime = loaded?.chime ?? true;
    form.uiScale = loaded?.uiScale ?? 100;
    form.uiScaleSteps = loaded?.uiScaleSteps ?? [];
    form.thoughts = fromView(loaded?.thoughts);
    form.pinned = { root: !!loaded?.pinned?.root, orgs: !!loaded?.pinned?.orgs };
    form.runners = loaded?.runners ?? [];
    loadedThoughts = { ...fromView(loaded?.thoughts), derived: loaded?.thoughts?.derived ?? "" };
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
        stickers: stickersInput(form.stickers),
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
    tab = firstErroredTab(fields) ?? tab;
    if (!err) {
      form.thoughts = fromView(toInput(form.thoughts));
      loadedThoughts = { ...fromView(toInput(form.thoughts)), derived: loadedThoughts.derived };
    }
    if (outcome.close) onClose();
  }

  async function checkGitHub() {
    if (checkingGitHub) return;
    checkingGitHub = true;
    const answer = await call(go.gitHubStatus());
    checkingGitHub = false;
    if (answer.ok) github = answer.value ?? null;
  }

  function clearStickerErrors() {
    fields = Object.fromEntries(Object.entries(fields).filter(([field]) => !field.startsWith("stickers.")));
  }

  function cancel() {
    onClose();
  }

  // A relaunch that works ends this process, so only a failure answers.
  async function relaunch() {
    const answer = await call(go.relaunch());
    if (!answer.ok) status = String(answer.error?.message ?? answer.error);
  }

  async function openConfig() {
    actionError = "";
    const answer = await call(go.openConfig());
    if (!answer.ok) actionError = String(answer.error?.message ?? answer.error);
  }

  async function installLinear() {
    if (installing) return;
    installing = true;
    const answer = await call(go.installLinear());
    installing = false;
    if (answer.ok) {
      linear = answer.value ?? linear;
      installed = true;
    } else {
      linear = { ...linear, error: String(answer.error?.message ?? answer.error) };
    }
  }

  return {
    form,
    get linear() {
      return linear;
    },
    get installed() {
      return installed;
    },
    get installing() {
      return installing;
    },
    get actionError() {
      return actionError;
    },
    get github() {
      return github;
    },
    get checkingGitHub() {
      return checkingGitHub;
    },
    get tab() {
      return tab;
    },
    set tab(value) {
      tab = value;
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
    clearStickerErrors,
    checkGitHub,
    relaunch,
    openConfig,
    installLinear,
  };
}
