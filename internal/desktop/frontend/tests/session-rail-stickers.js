const DEFAULTS = [
  { id: "star", colour: "blue", symbol: "star", label: "Important" },
  { id: "bookmark", colour: "green", symbol: "bookmark", label: "Read later" },
  { id: "question", colour: "orange", symbol: "question", label: "Needs follow-up" },
  { id: "exclamation", colour: "red", symbol: "exclamation", label: "Has bugs" },
];

/** @param {Record<string, string>} labels */
export const labelled = (labels) => DEFAULTS.map((item) => ({ ...item, label: labels[item.id] ?? item.label }));
