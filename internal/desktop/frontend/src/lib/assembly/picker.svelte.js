import { call } from "../wails.js";
import { browsing } from "./browse.svelte.js";
import * as go from "./calls.js";
import { refusal } from "./steps.js";

/**
 * picking is one visit to the picker over a live session: the rows it already
 * holds, the branch anything added joins, and the answer an escalation waiting
 * on it gets. Nothing is committed until confirm, so a visit that ends any other
 * way leaves the session as it was. done is called once Go has the answer.
 * @param {() => string} slug
 * @param {() => void} done
 */
export function picking(slug, done) {
  let branch = $state("");
  let status = $state("");
  let answering = $state(false);

  /** @param {string} text */
  const report = (text) => (status = text);

  const repos = browsing(() => branch, report);

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
    const picked = { repos: repos.ordered, upgrades: repos.upgrading };
    const added = await call(go.addRepos(slug(), picked));
    if (!added.ok) {
      answering = false;
      status = refusal(added.error);
      return;
    }
    done();
  }

  async function cancel() {
    if (answering) return;
    answering = true;
    const cancelled = await call(go.cancelPicker(slug()));
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
    confirm,
    cancel,
  };
}
