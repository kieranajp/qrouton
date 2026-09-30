import { artifactTone } from "./artifacts.js";
import { ARTIFACT_KINDS } from "./bridge/generated.js";

const TONES = {
  RESEARCH: artifactTone(ARTIFACT_KINDS.RESEARCH),
  PLAN: artifactTone(ARTIFACT_KINDS.PLAN),
  IMPLEMENT: "var(--state-success)",
};

/** @param {"RESEARCH" | "PLAN" | "IMPLEMENT"} stage */
export const workflowTone = (stage) => TONES[stage];
