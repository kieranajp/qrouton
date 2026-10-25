import { CHROME_AGENT_TREE } from "../bridge/generated.js";
import { Call, call } from "../wails.js";

/** @param {string} slug */
export const agentTree = (slug) => call(Call.ByName(CHROME_AGENT_TREE, slug));
