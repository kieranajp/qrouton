export const Browser = { OpenURL: async () => {} };
export const Clipboard = {
  SetText: async (text) => {
    window.clipboardText = text;
  },
};
export const Events = { On: () => () => {} };
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
