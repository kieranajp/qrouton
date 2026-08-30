/** @typedef {{start: string, write: string, resize: string, data: string, exit: string}} PTY */

// Whole names, not a base the callers append to: the Go side greps the built
// page for every bound method, and a name assembled across a module boundary
// survives bundling as a concatenation it cannot find.
/** @type {PTY} */
export const conversationPTY = {
  start: "github.com/kieranajp/qrouton/internal/desktop.Term.Start",
  write: "github.com/kieranajp/qrouton/internal/desktop.Term.Write",
  resize: "github.com/kieranajp/qrouton/internal/desktop.Term.Resize",
  data: "pty:data:",
  exit: "pty:exit:",
};

/** @type {PTY} */
export const tabPTY = {
  start: "github.com/kieranajp/qrouton/internal/desktop.Windows.Start",
  write: "github.com/kieranajp/qrouton/internal/desktop.Windows.Write",
  resize: "github.com/kieranajp/qrouton/internal/desktop.Windows.Resize",
  data: "window:data:",
  exit: "window:exit:",
};

export const SURFACES = "github.com/kieranajp/qrouton/internal/desktop.Windows.Surfaces";
export const CLOSE_WINDOW = "github.com/kieranajp/qrouton/internal/desktop.Windows.Close";
export const OPEN_SHELL = "github.com/kieranajp/qrouton/internal/desktop.Windows.OpenShell";
export const SELECT_WINDOW = "github.com/kieranajp/qrouton/internal/desktop.Windows.Select";
export const OPEN_DOCUMENT = "github.com/kieranajp/qrouton/internal/desktop.Windows.OpenDocument";
