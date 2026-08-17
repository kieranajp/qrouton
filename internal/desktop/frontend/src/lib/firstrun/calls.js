import { Call } from "../wails.js";

const FIRST_RUN_SERVICE = "github.com/kieranajp/qrouton/internal/desktop.FirstRun";

/**
 * @param {{orgs: string[], root: string}} input
 * @returns {Promise<{relaunching: boolean}>}
 */
export const save = (input) => Call.ByName(FIRST_RUN_SERVICE + ".Save", input);
