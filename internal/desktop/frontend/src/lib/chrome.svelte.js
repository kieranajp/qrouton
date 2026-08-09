import { Events } from "./wails.js";

/** @type {{mode: string, phase: string, identity: string, branch: string,
 *   sessions: any[], documents: any[], windows: any[], repos: any[],
 *   activity: 'working'|'waiting'|'idle'}} */
const NOTHING = {
  mode: "",
  phase: "",
  identity: "",
  branch: "",
  sessions: [],
  documents: [],
  windows: [],
  repos: [],
  activity: "idle",
};

/** chrome is the last thing the workbench said it could observe. */
export function chrome() {
  let fields = $state(NOTHING);
  Events.On("chrome:update", (event) => {
    fields = { ...NOTHING, ...(event.data || {}) };
  });
  return {
    get fields() {
      return fields;
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
