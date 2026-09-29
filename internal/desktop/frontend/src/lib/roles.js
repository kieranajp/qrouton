// What a session does with a repository, in the marks and words every list of
// them uses.

import { REPO_ROLES } from "./bridge/generated.js";

export const GLYPHS = { [REPO_ROLES.EDITING]: "●", [REPO_ROLES.REFERENCE]: "◐" };

export const READ_ONLY = "read-only";
