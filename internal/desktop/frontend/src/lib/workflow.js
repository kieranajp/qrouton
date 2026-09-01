import { artifactTone } from "./artifacts.js";

const TONES = {
  RESEARCH: artifactTone("RESEARCH"),
  PLAN: artifactTone("PLAN"),
  IMPLEMENT: "var(--state-success)",
};

/** @param {"RESEARCH" | "PLAN" | "IMPLEMENT"} stage */
export const workflowTone = (stage) => TONES[stage];
