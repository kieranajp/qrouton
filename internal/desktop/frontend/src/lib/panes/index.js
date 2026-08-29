import DiffPane from "./DiffPane.svelte";
import MarkdownPane from "./MarkdownPane.svelte";
import PlainPane from "./PlainPane.svelte";
import PlanPane from "./PlanPane.svelte";

// A pane per document format. The window declares its format; guessing it from
// the text would paint a plain document that quotes a diff as one.
const PANES = {
  diff: DiffPane,
  markdown: MarkdownPane,
};

// Some markdown is more than markdown. The kind is the workbench's own reading
// of where the file lives, so it refines the format rather than replacing it.
const KINDS = {
  PLAN: PlanPane,
};

export const paneFor = (format, kind) => {
  const pane = PANES[format] ?? PlainPane;
  return pane === MarkdownPane ? (KINDS[kind] ?? pane) : pane;
};
