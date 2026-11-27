import { ASSEMBLY_PROGRESS_EVENT } from "../bridge/generated.js";
import { Events } from "../wails.js";
import { record } from "./progress.js";

/** Every page in the process hears every session's progress, so rows keep only
 * the events for the session the getter names.
 * @param {() => string} session */
export function following(session) {
  let rows = $state(/** @type {import("./progress.js").Row[]} */ ([]));

  $effect(() =>
    Events.On(ASSEMBLY_PROGRESS_EVENT, (event) => {
      const advance = event.data ?? {};
      if (advance.session !== session()) return;
      rows = record(rows, advance);
    }),
  );

  return {
    get rows() {
      return rows;
    },
    clear() {
      rows = [];
    },
  };
}
