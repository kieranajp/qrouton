import { UPDATE_EVENT, UPDATES_STATUS } from "./bridge/generated.js";
import { call, Call, follow } from "./wails.js";

const NOTHING = { available: false, current: "", latest: "", command: "", url: "" };

/** updates is the last known answer to whether this build is outdated. */
export function updates() {
  return (shared ??= observing());
}

let shared;

function observing() {
  let status = $state(NOTHING);
  // A failed snapshot reads as not available, same as a check Go itself
  // could not complete.
  follow(UPDATE_EVENT, () => call(Call.ByName(UPDATES_STATUS)), (value) => {
    status = value ?? NOTHING;
  });
  return {
    get status() {
      return status;
    },
  };
}
