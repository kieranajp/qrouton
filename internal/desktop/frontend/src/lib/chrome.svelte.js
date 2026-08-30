import { CHROME_DEFAULTS, CHROME_EVENT, CHROME_SNAPSHOT } from "./bridge/generated.js";
import { call, Call, Events } from "./wails.js";

// Idle rather than the payload's zero value: a page that has heard nothing is
// not a page reporting an agent at work.
const NOTHING = { ...CHROME_DEFAULTS, activity: "idle" };

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
  Events.On(CHROME_EVENT, (event) => {
    live = true;
    apply(event.data);
  });
  // A workbench that cannot say what it holds reads as one holding no session,
  // which is a window with something on it rather than a permanently blank one.
  call(Call.ByName(CHROME_SNAPSHOT)).then((answer) => {
    if (!live) apply(answer.ok ? answer.value : undefined);
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
