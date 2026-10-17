import {
  OLLAMA_CANCEL_PULL,
  OLLAMA_CHECK,
  OLLAMA_EVENT,
  OLLAMA_PULL,
  OLLAMA_URL,
} from "./bridge/generated.js";
import { call, Call, follow } from "./wails.js";

const UNKNOWN = { state: "", model: "" };

export const DOWNLOAD_URL = "https://ollama.com/download";

export const host = new URL(OLLAMA_URL).host;

/** @param {{completed?: number, total?: number}} status */
export const percent = (status) => (status.total ? Math.round(((status.completed ?? 0) / status.total) * 100) : 0);

/** @param {number} [bytes] */
export const megabytes = (bytes = 0) => `${(bytes / 1e6).toFixed(1)} MB`;

/** ollama is the last known state of semantic search, shared by every view of it.
 * Each view checks again when it mounts, so an old answer does not linger. */
export function ollama() {
  if (shared) shared.check();
  else shared = observing();
  return shared;
}

let shared;

function observing() {
  let status = $state(UNKNOWN);
  // A failed check keeps the last status rather than forgetting it.
  const { refresh } = follow(OLLAMA_EVENT, () => call(Call.ByName(OLLAMA_CHECK)), (value) => (status = value ?? UNKNOWN), {
    settle: (answer, live) => {
      if (answer.ok && answer.value && !live) status = answer.value;
    },
  });
  return {
    get status() {
      return status;
    },
    check: refresh,
    pull: () => call(Call.ByName(OLLAMA_PULL)),
    cancel: () => call(Call.ByName(OLLAMA_CANCEL_PULL)),
  };
}
