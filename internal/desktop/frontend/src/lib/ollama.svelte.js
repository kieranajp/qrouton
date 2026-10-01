import {
  OLLAMA_CANCEL_PULL,
  OLLAMA_CHECK,
  OLLAMA_EVENT,
  OLLAMA_PULL,
} from "./bridge/generated.js";
import { call, Call, Events } from "./wails.js";

const UNKNOWN = { state: "", model: "" };

/** ollama is the last known state of semantic search, shared by every view of it.
 * Each view checks again when it mounts, so an old answer does not linger. */
export function ollama() {
  return (shared ??= observing());
}

let shared;

function observing() {
  let status = $state(UNKNOWN);
  let events = 0;
  Events.On(OLLAMA_EVENT, (event) => {
    events++;
    status = event.data ?? UNKNOWN;
  });
  // An event that lands while a check is out is newer than the check's answer.
  const check = () => {
    const before = events;
    call(Call.ByName(OLLAMA_CHECK)).then((answer) => {
      if (answer.ok && answer.value && events === before) status = answer.value;
    });
  };
  return {
    get status() {
      return status;
    },
    check,
    pull: () => call(Call.ByName(OLLAMA_PULL)),
    cancel: () => call(Call.ByName(OLLAMA_CANCEL_PULL)),
  };
}
