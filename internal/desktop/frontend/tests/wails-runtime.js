export const Browser = {
  OpenURL: async (url) => {
    window.openedURL = url;
  },
};
export const Clipboard = {
  SetText: async (text) => {
    window.clipboardText = text;
  },
  Text: async () => window.clipboardText ?? "",
};
const listeners = new Map();
export const Events = {
  On: (name, listener) => {
    const eventListeners = listeners.get(name) ?? new Set();
    eventListeners.add(listener);
    listeners.set(name, eventListeners);
    return () => eventListeners.delete(listener);
  },
};
export const emitWailsEvent = (name, data) => {
  for (const listener of listeners.get(name) ?? []) listener({ data });
};
export const Call = {
  ByName: async (name, ...args) => {
    if (window.wailsCall) return window.wailsCall(name, ...args);
    return name.endsWith(".Content")
      ? {
          text: "# Find me\n\nFind **me** once. Find me twice.",
          format: "markdown",
          source: "notes/find.md",
          path: "/sessions/artifacts/notes/find.md",
          kind: "PLAN",
          line: 0,
          to: 0,
        }
      : undefined;
  },
};
