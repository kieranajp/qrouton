import { Call } from "../wails.js";

const FIRST_RUN_SERVICE = "github.com/kieranajp/qrouton/internal/desktop.FirstRun";

/**
 * @param {{orgs: string[], root: string}} input
 * @returns {Promise<{relaunching: boolean}>}
 */
export const save = (input) => Call.ByName(FIRST_RUN_SERVICE + ".Save", input);

/** login is the signed-in GitHub account, or "" when there is none. @returns {Promise<string>} */
export const login = () => Call.ByName(FIRST_RUN_SERVICE + ".Login");

/** chooseRoot prompts for a directory, answering "" on a cancel. @returns {Promise<string>} */
export const chooseRoot = () => Call.ByName(FIRST_RUN_SERVICE + ".ChooseRoot");
