import { call } from "../wails.js";
import { browsing } from "./browse.svelte.js";
import * as go from "./calls.js";
import { following } from "./following.svelte.js";
import { refusal } from "./steps.js";

/** Picker state is provisional until confirmation reaches Go.
 * @param {() => string} slug
 * @param {() => string} requestID the escalation answered, empty for a plain add
 * @param {() => void} done */
export function picking(slug, requestID, done) {
  let branch = $state("");
  let status = $state("");
  let answering = $state(false);
  let adding = $state(false);
  let running = false;
  let aborting = false;

  /** @param {string} text */
  const report = (text) => (status = text);

  const repos = browsing(() => branch, report);
  const progress = following(slug);

  call(go.held(slug())).then((answer) => {
    if (!answer.ok) return report(refusal(answer.error));
    branch = answer.value?.branch ?? "";
    repos.hold(answer.value?.repos ?? []);
  });

  // answering stays set once Go has the answer: the manifest now holds what this
  // visit picked, and asking again would compose it a second time.
  async function confirm() {
    if (answering) return;
    answering = true;
    adding = true;
    running = true;
    progress.clear();
    const picked = { repos: repos.ordered, upgrades: repos.upgrading };
    const added = await call(go.addRepos(slug(), requestID(), picked));
    running = false;
    aborting = false;
    if (!added.ok) {
      answering = false;
      adding = false;
      status = refusal(added.error);
      return;
    }
    done();
  }

  // An aborted add answers through its own pending call, which restores the list.
  async function abort() {
    if (aborting) return;
    aborting = true;
    const aborted = await call(go.abortAdd(slug()));
    if (!aborted.ok) {
      aborting = false;
      status = refusal(aborted.error);
    }
  }

  async function cancel() {
    if (running) return abort();
    if (answering) return;
    answering = true;
    const cancelled = await call(go.cancelPicker(slug(), requestID()));
    if (!cancelled.ok) {
      answering = false;
      status = refusal(cancelled.error);
      return;
    }
    done();
  }

  return {
    repos,
    get branch() {
      return branch;
    },
    get status() {
      return status;
    },
    get answering() {
      return answering;
    },
    get adding() {
      return adding;
    },
    get progress() {
      return progress.rows;
    },
    confirm,
    cancel,
  };
}
