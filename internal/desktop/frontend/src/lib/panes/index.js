import DiffPane from "./DiffPane.svelte";
import MarkdownPane from "./MarkdownPane.svelte";
import PlainPane from "./PlainPane.svelte";
import PlanPane from "./PlanPane.svelte";
import ResearchPane from "./ResearchPane.svelte";
import SlidesPane from "./SlidesPane.svelte";

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
  RESEARCH: ResearchPane,
};

// A deck is a presentation form, not an artifact kind, so it is asked first and
// answered from its own field: a spec can be deck-shaped and stay a spec.
export const paneFor = (doc) => {
  if (doc.deck) return SlidesPane;
  const pane = PANES[doc.format] ?? PlainPane;
  return pane === MarkdownPane ? (KINDS[doc.kind] ?? pane) : pane;
};
