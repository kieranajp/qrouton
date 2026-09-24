import { UPDATE_EVENT, UPDATES_STATUS } from "./bridge/generated.js";
import { call, Call, Events } from "./wails.js";

const NOTHING = { available: false, current: "", latest: "", command: "", url: "" };

/** updates is the last known answer to whether this build is outdated. */
export function updates() {
  return (shared ??= observing());
}

let shared;

function observing() {
  let status = $state(NOTHING);
  let live = false;
  const apply = (value) => {
    status = value ?? NOTHING;
  };
  Events.On(UPDATE_EVENT, (event) => {
    live = true;
    apply(event.data);
  });
  // A failed snapshot reads as not available, same as a check Go itself
  // could not complete.
  call(Call.ByName(UPDATES_STATUS)).then((answer) => {
    if (!live) apply(answer.ok ? answer.value : undefined);
  });
  return {
    get status() {
      return status;
    },
  };
}
