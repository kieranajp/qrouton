import { artifactTone } from "./artifacts.js";
import { AGENT_STAGES, ARTIFACT_KINDS } from "./bridge/generated.js";

const TONES = {
  RESEARCH: artifactTone(ARTIFACT_KINDS.RESEARCH),
  PLAN: artifactTone(ARTIFACT_KINDS.PLAN),
  IMPLEMENT: "var(--state-success)",
};

/** @param {"RESEARCH" | "PLAN" | "IMPLEMENT"} stage */
export const workflowTone = (stage) => TONES[stage];

/** @param {string | undefined} stage an AGENT_STAGES value */
export const stageTone = (stage) =>
  workflowTone(/** @type {"RESEARCH" | "PLAN" | "IMPLEMENT"} */ (
    Object.keys(AGENT_STAGES).find((key) => AGENT_STAGES[/** @type {keyof typeof AGENT_STAGES} */ (key)] === stage)
  ));
