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

  const repos = browsing(() => branch);

  go.held(slug())
    .then((held) => {
      branch = held?.branch ?? "";
      repos.hold(held?.repos ?? []);
    })
    .catch((err) => (status = refusal(err)));

  // answering stays set once Go has the answer: the manifest now holds what this
  // visit picked, and asking again would compose it a second time.
  async function confirm() {
    if (answering) return;
    answering = true;
    try {
      await go.addRepos(slug(), { repos: repos.ordered, upgrades: repos.upgrading });
      done();
    } catch (err) {
      answering = false;
      status = refusal(err);
    }
  }

  async function cancel() {
    if (answering) return;
    answering = true;
    try {
      await go.cancelPicker(slug());
      done();
    } catch (err) {
      answering = false;
      status = refusal(err);
    }
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
