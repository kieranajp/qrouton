import { CHROME_DEFAULTS, CHROME_EVENT, CHROME_SNAPSHOT, ACTIVITIES } from "./bridge/generated.js";
import { call, Call, follow } from "./wails.js";

// Idle rather than the payload's zero value: a page that has heard nothing is
// not a page reporting an agent at work.
const NOTHING = { ...CHROME_DEFAULTS, activity: ACTIVITIES.IDLE };

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
  // A workbench that cannot say what it holds reads as one holding no session,
  // which is a window with something on it rather than a permanently blank one.
  follow(CHROME_EVENT, () => call(Call.ByName(CHROME_SNAPSHOT)), (value) => {
    fields = observed(value);
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
