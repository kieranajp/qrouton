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

/** age is how long ago something was written, never more precise than it is. */
export function age(at) {
  const minutes = Math.floor((Date.now() - new Date(at).getTime()) / 60000);
  if (minutes < 1) return "just now";
  if (minutes < 60) return `${minutes}m ago`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours}h ago`;
  return `${Math.floor(hours / 24)}d ago`;
}
