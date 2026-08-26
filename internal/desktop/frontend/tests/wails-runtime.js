export const Browser = { OpenURL: async () => {} };
export const Events = { On: () => () => {} };
export const Call = {
  ByName: async (name, ...args) => {
    if (window.wailsCall) return window.wailsCall(name, ...args);
    return name.endsWith(".Content")
      ? {
          text: "# Find me\n\nFind **me** once. Find me twice.",
          format: "markdown",
          source: "notes/find.md",
          line: 0,
          to: 0,
        }
      : undefined;
  },
};
