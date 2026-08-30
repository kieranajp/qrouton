import { debounced } from "../async.js";
import { call, Events } from "../wails.js";
import { browsing } from "./browse.svelte.js";
import * as go from "./calls.js";
import { record } from "./progress.js";
import { blocks, folder, last, refusal } from "./steps.js";
import { loader } from "./ticket.js";

const TICKET_LOADED = "✓ Loaded — name and description filled in";
const NO_RUNNERS = "No agent was found on your PATH, so there is nothing to run this session with.";

// Long enough that a typed word is one branch preview rather than six.
const PREVIEW_DELAY = 150;

// A ticket tracker's answer, which names no field of the form.
const message = (err) => String(err?.message ?? err ?? "");

/**
 * assembling is one run through the overlay: the state the three steps share,
 * with every rule left to the modules beside this one and to Go. done is called
 * once the session exists, because the workbench brings it on screen itself.
 * @param {() => void} done
 */
export function assembling(done) {
  const form = $state({
    name: "",
    entropy: "",
    description: "",
    ticket: "",
    prefix: "",
    mode: "rpi",
    runner: "",
  });

  let step = $state(0);
  let prefixes = $state(/** @type {string[]} */ ([]));
  let runners = $state(/** @type {{id: string, label: string}[]} */ ([]));
  let branch = $state("");
  let status = $state("");
  let failure = $state("");
  /** @type {{text: string, tone: 'muted'|'success'|'failed'}} */
  let hint = $state({ text: "", tone: "success" });
  let fetching = $state(false);
  let creating = $state(false);
  let progress = $state(/** @type {import("./progress.js").Row[]} */ ([]));

  /** @param {string} text */
  const report = (text) => (failure ||= text);

  const repos = browsing(() => branch, report);

  call(go.prefixes()).then((answer) => {
    if (!answer.ok) return report(refusal(answer.error));
    prefixes = answer.value ?? [];
    form.prefix = form.prefix || prefixes[0] || "";
  });
  call(go.runners()).then((answer) => {
    if (!answer.ok) return report(refusal(answer.error));
    runners = answer.value ?? [];
    form.runner = form.runner || runners[0]?.id || "";
    if (!runners.length) report(NO_RUNNERS);
  });

  const payload = () => ({ ...form, repos: repos.ordered });

  const branches = debounced(
    PREVIEW_DELAY,
    (draft) => call(go.preview(draft)),
    (answer) => {
      if (!answer.ok) return report(refusal(answer.error));
      branch = answer.value ?? "";
    },
  );

  // The branch is the prefix and the slug; nothing else on the form moves it, and
  // a description that re-asked would be a round trip per keystroke.
  $effect(() => {
    const { name, entropy, prefix } = form;
    branches.schedule({ name, entropy, prefix });
    return branches.cancel;
  });

  // Every page in the process hears this, and the slug the session will have is
  // the folder the previewed branch names.
  $effect(() =>
    Events.On("assembly:progress", (event) => {
      const advance = event.data ?? {};
      if (advance.session !== folder(branch)) return;
      progress = record(progress, advance);
    }),
  );

  const tickets = loader(form, go.fetchTicket, {
    fetching(active) {
      fetching = active;
    },
    loaded(filled) {
      form.name = filled.name;
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
      return progress;
    },
    back,
    next,
    load,
    seed,
  };
}
