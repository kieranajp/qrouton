import { Call, Events } from "./wails.js";

const CHROME_SERVICE = "github.com/kieranajp/qrouton/internal/desktop.Chrome";

/** @type {{mode: string, phase: string, identity: string, branch: string,
 *   slug: string, terminal: string,
 *   sessions: any[], documents: any[], repositoryDocuments: any[], repos: any[],
 *   activity: 'working'|'waiting'|'idle', agents: {provider: string, agents: any[]},
 *   picker: boolean, welcoming: boolean}} */
const NOTHING = {
  mode: "",
  phase: "",
  identity: "",
  branch: "",
  slug: "",
  terminal: "",
  sessions: [],
  documents: [],
  repositoryDocuments: [],
  repos: [],
  activity: "idle",
  agents: { provider: "", agents: [] },
  picker: false,
  welcoming: false,
};

// Spreading over the defaults is not enough: a slice Go leaves nil arrives as
// an explicit null, which fills the key rather than leaving it absent, and the
// first .length on it unwinds the render before the rest of the fields draw.
const observed = (data) =>
  /** @type {typeof NOTHING} */ (
    Object.fromEntries(
      Object.entries(NOTHING).map(([key, fallback]) => [key, data?.[key] ?? fallback]),
    )
  );

/** chrome is the last thing the workbench said it could observe. */
export function chrome() {
  return (shared ??= observing());
}

// One subscription for the whole page: every caller wants the same live fields,
// and a second Events.On would outlive the component that opened it.
let shared;

function observing() {
  let fields = $state(NOTHING);
  let settled = $state(false);
  let live = false;
  const apply = (value) => {
    fields = observed(value);
    settled = true;
  };
  Events.On("chrome:update", (event) => {
    live = true;
    apply(event.data);
  });
  Call.ByName(CHROME_SERVICE + ".Snapshot").then((value) => {
    if (!live) apply(value);
  });
  return {
    get fields() {
      return fields;
    },
    // Until the first payload arrives every field reads as its default, which is
    // indistinguishable from a window holding no session.
    get settled() {
      return settled;
    },
  };
}
