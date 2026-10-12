import { PLACEMENT_REASONS } from "../bridge/generated.js";

const labels = {
  [PLACEMENT_REASONS.AGENT_RUNNING]: "An agent is running in it",
  [PLACEMENT_REASONS.DESTINATION_MISSING]: "The destination folder is missing",
  [PLACEMENT_REASONS.CROSS_DEVICE]: "It would move between volumes; move it by hand",
  [PLACEMENT_REASONS.LINK_BROKEN]: "Its thoughts link is broken",
  [PLACEMENT_REASONS.CONFLICT]: "The destination already holds a folder for it",
    [PLACEMENT_REASONS.UNREADABLE]: "Its manifest cannot be read",
};

/** @param {import("./calls.js").PlacementItem} item */
export const reasonLabel = (item) =>
  item.reason === PLACEMENT_REASONS.OUTSIDE_ROOTS
    ? `Its thoughts live in ${item.from.path}, outside your configured folders. Move them by hand`
    : (item.reason && labels[item.reason]) || "It cannot be moved safely";
