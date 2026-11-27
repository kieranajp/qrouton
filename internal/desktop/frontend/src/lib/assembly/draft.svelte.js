import { debounced } from "../async.js";
import { SESSION_MODES } from "../bridge/generated.js";
import { profileStatus } from "../settings/calls.js";
import { byProfile } from "../settings/profiles.js";
import { call } from "../wails.js";
import { browsing } from "./browse.svelte.js";
import * as go from "./calls.js";
import { following } from "./following.svelte.js";
import { blocks, folder, last, refusal } from "./steps.js";
import { loader } from "./ticket.js";

const TICKET_LOADED = "Loaded — session and branch details filled in";
const NO_RUNNERS = "No agent was found on your PATH, so there is nothing to run this session with.";

// Long enough that a typed word is one branch preview rather than six.
const PREVIEW_DELAY = 150;

// A ticket tracker's answer, which names no field of the form.
const message = (err) => String(err?.message ?? err ?? "");

/** @param {() => void} done */
export function assembling(done) {
  const form = $state({
    name: "",
    branchDescription: "",
    entropy: "",
    description: "",
    ticket: "",
    prefix: "",
    mode: SESSION_MODES.RPI,
    runner: "",
    profile: "",
  });

  let step = $state(0);
  let prefixes = $state(/** @type {string[]} */ ([]));
  let runners = $state(/** @type {go.RunnerRow[]} */ ([]));
  let statuses = $state(/** @type {Record<string, import("../settings/calls.js").ProfileStatus> | null} */ (null));
  let branch = $state("");
  let status = $state("");
  let failure = $state("");
  /** @type {{text: string, tone: 'muted'|'success'|'failed'}} */
  let hint = $state({ text: "", tone: "success" });
  let fetching = $state(false);
  let creating = $state(false);

  /** @param {string} text */
  const report = (text) => (failure ||= text);

  const repos = browsing(() => branch, report);

  call(go.prefixes()).then((answer) => {
    if (!answer.ok) return report(refusal(answer.error));
    prefixes = answer.value ?? [];
    form.prefix = form.prefix || prefixes[0] || "";
  });
  // Settings can add or delete a profile while the wizard sits beneath it, so a
  // choice that has gone falls back to the first row.
  async function readRunners() {
    const answer = await call(go.runners());
    if (!answer.ok) return report(refusal(answer.error));
    runners = answer.value ?? [];
    if (!runners.some((row) => row.id === form.runner && row.profile === form.profile)) {
      form.runner = runners[0]?.id ?? "";
      form.profile = runners[0]?.profile ?? "";
    }
    if (!runners.length) report(NO_RUNNERS);
    if (runners.some((row) => row.profile)) probe();
  }

  async function probe() {
    const answer = await call(profileStatus());
    if (answer.ok) statuses = byProfile(answer.value);
  }

  readRunners();

  const payload = () => ({ ...form, repos: repos.ordered });

  const branches = debounced(
    PREVIEW_DELAY,
    (draft) => call(go.preview(draft)),
    (answer) => {
      if (!answer.ok) return report(refusal(answer.error));
      branch = answer.value ?? "";
    },
  );

  // Prose and repository choices do not move the branch, so they do not ask for
  // another preview on every keystroke.
  $effect(() => {
    const { name, branchDescription, ticket, entropy, prefix } = form;
    branches.schedule({ name, branchDescription, ticket, entropy, prefix });
    return branches.cancel;
  });

  // The slug the session will have is the folder the previewed branch names.
  const progress = following(() => folder(branch));

  const tickets = loader(form, go.fetchTicket, {
    fetching(active) {
      fetching = active;
    },
    loaded(filled) {
      form.name = filled.name;
      form.branchDescription = filled.branchDescription;
      form.description = filled.description;
      hint = { text: TICKET_LOADED, tone: "success" };
    },
    failed(err) {
      hint = { text: message(err), tone: "failed" };
    },
  });

  const load = () => tickets.load();

  /** @param {string} ticket */
  const seed = (ticket) => tickets.seed(ticket);

  // Nothing reads the rules between advances, so they are asked for on the press
  // that acts on them rather than on every keystroke.
  async function next() {
    if (creating) return;
    const found = await call(go.check(payload()));
    if (!found.ok) {
      status = refusal(found.error);
      return;
    }
    const blocked = blocks(found.value ?? [], step);
    if (blocked) {
      status = blocked.message;
      return;
    }
    status = "";
    if (step === 0) {
      const slug = await call(go.checkSlug(payload()));
      if (!slug.ok) {
        status = refusal(slug.error);
        return;
      }
      if (slug.value?.length) {
        status = slug.value[0].message;
        return;
      }
    }
    if (step < last) {
      step += 1;
      return;
    }
    creating = true;
    const created = await call(go.create(payload()));
    if (!created.ok) {
      creating = false;
      status = refusal(created.error);
      return;
    }
    done();
  }

  function back() {
    status = "";
    if (step > 0) step -= 1;
  }

  return {
    form,
    repos,
    get step() {
      return step;
    },
    get prefixes() {
      return prefixes;
    },
    get runners() {
      return runners;
    },
    get statuses() {
      return statuses;
    },
    get branch() {
      return branch;
    },
    // A step clears what it was told off for; what could not be loaded at all
    // outlives the step the user was on when it failed.
    get status() {
      return status || failure;
    },
    get hint() {
      return hint;
    },
    get fetching() {
      return fetching;
    },
    get creating() {
      return creating;
    },
    get progress() {
      return progress.rows;
    },
    back,
    next,
    load,
    seed,
    readRunners,
  };
}
