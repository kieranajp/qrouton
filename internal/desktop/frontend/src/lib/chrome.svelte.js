import { Events } from "./wails.js";

/** @type {{mode: string, phase: string, identity: string, branch: string,
 *   slug: string, terminal: string,
 *   sessions: any[], documents: any[], repos: any[],
 *   activity: 'working'|'waiting'|'idle', picker: boolean, welcoming: boolean}} */
const NOTHING = {
  mode: "",
  phase: "",
  identity: "",
  branch: "",
  slug: "",
  terminal: "",
  sessions: [],
  documents: [],
  repos: [],
  activity: "idle",
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
  Events.On("chrome:update", (event) => {
    fields = observed(event.data);
    settled = true;
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
