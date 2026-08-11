import DiffPane from "./DiffPane.svelte";
import MarkdownPane from "./MarkdownPane.svelte";
import PlainPane from "./PlainPane.svelte";

// A pane per document format. The window declares its format; guessing it from
// the text would paint a plain document that quotes a diff as one.
const PANES = {
  diff: DiffPane,
  markdown: MarkdownPane,
};

export const paneFor = (format) => PANES[format] ?? PlainPane;
