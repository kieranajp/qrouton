import { FIRST_RUN_CHOOSE_ROOT, FIRST_RUN_LOGIN, FIRST_RUN_SAVE } from "../bridge/generated.js";
import { Call } from "../wails.js";

/**
 * @param {{orgs: string[], root: string}} input
 * @returns {Promise<{relaunching: boolean}>}
 */
export const save = (input) => Call.ByName(FIRST_RUN_SAVE, input);

/** login is the signed-in GitHub account, or "" when there is none. @returns {Promise<string>} */
export const login = () => Call.ByName(FIRST_RUN_LOGIN);

/** chooseRoot prompts for a directory, answering "" on a cancel. @returns {Promise<string>} */
export const chooseRoot = () => Call.ByName(FIRST_RUN_CHOOSE_ROOT);
