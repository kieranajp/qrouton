import { Events } from "../wails.js";
import { browsing } from "./browse.svelte.js";
import * as go from "./calls.js";
import { record } from "./progress.js";
import { blocks, folder, last, refusal } from "./steps.js";
import { loader } from "./ticket.js";

const TICKET_LOADED = "✓ Loaded — name and description filled in";

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
    description: "",
    ticket: "",
    prefix: "",
    mode: "rpi",
    runner: "",
  });

  let step = $state(0);
  let prefixes = $state(/** @type {string[]} */ ([]));
  let runners = $state(/** @type {{id: string, label: string}[]} */ ([]));
  let problems = $state(/** @type {{field: string, message: string}[]} */ ([]));
  let branch = $state("");
  let status = $state("");
  /** @type {{text: string, tone: 'muted'|'success'|'failed'}} */
  let hint = $state({ text: "", tone: "success" });
  let fetching = $state(false);
  let creating = $state(false);
  let progress = $state(/** @type {import("./progress.js").Row[]} */ ([]));

  const repos = browsing(() => branch);

  go.prefixes().then((list) => {
    prefixes = list ?? [];
    form.prefix = form.prefix || prefixes[0] || "";
  });
  go.runners().then((list) => {
    runners = list ?? [];
    form.runner = form.runner || runners[0]?.id || "";
  });

  const payload = () => ({ ...form, repos: repos.ordered });

  // Only the newest answer may land: two keystrokes are two calls, and they
  // come back in whatever order Go finishes them.
  let asked = 0;
  $effect(() => {
    const draft = payload();
    const mine = ++asked;
    go.check(draft).then((found) => {
      if (mine === asked) problems = found ?? [];
    });
    go.preview(draft).then((value) => {
      if (mine === asked) branch = value ?? "";
    });
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

  async function next() {
    if (creating) return;
    const blocked = blocks(problems, step);
    if (blocked) {
      status = blocked.message;
      return;
    }
    status = "";
    if (step === 0) {
      const found = await go.checkSlug(payload());
      if (found?.length) {
        status = found[0].message;
        return;
      }
    }
    if (step < last) {
      step += 1;
      return;
    }
    creating = true;
    try {
      await go.create(payload());
      done();
    } catch (err) {
      creating = false;
      status = refusal(err);
    }
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
    get status() {
      return status;
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
