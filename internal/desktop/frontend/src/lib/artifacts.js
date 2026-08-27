const TONES = {
  PLAN: "var(--artifact-plan)",
  SPEC: "var(--artifact-spec)",
  RESEARCH: "var(--artifact-research)",
  NOTE: "var(--artifact-note)",
};

/** @param {string | undefined} kind */
export const artifactTone = (kind) => TONES[kind ?? ""] ?? TONES.NOTE;
