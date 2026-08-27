import { resolve } from "node:path";
import { defineConfig } from "vite";

// The bundle behind a shared document: the workbench's own renderer and prose
// styles, built for a file that must open with nothing to fetch. Separate from
// vite.config.js because the two disagree about assets — the workbench serves
// its fonts, this page carries them.
// The markdown parser carries U+FFFD literally, to stand in for input it
// rejects. A page is published by uploading it, and a validator that meets a
// replacement character reasonably assumes it is reading something already
// broken — so the character is escaped to the sequence that means it.
const escapeReplacementChar = {
  name: "qrouton-escape-replacement-char",
  generateBundle: (_, bundle) => {
    for (const chunk of Object.values(bundle)) {
      if (chunk.type === "chunk") chunk.code = chunk.code.replaceAll("\uFFFD", "\\uFFFD");
    }
  },
};

export default defineConfig({
  plugins: [escapeReplacementChar],
  build: {
    outDir: "../../share/assets",
    emptyOutDir: true,
    cssCodeSplit: false,
    // A page under a strict CSP cannot request a font file, so every asset is
    // inlined however large it is.
    assetsInlineLimit: () => true,
    rollupOptions: {
      input: resolve(import.meta.dirname, "src/share/main.js"),
      output: {
        format: "iife",
        entryFileNames: "share.js",
        assetFileNames: "share.css",
      },
    },
  },
});
