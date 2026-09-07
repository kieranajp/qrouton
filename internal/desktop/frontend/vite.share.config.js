import { resolve } from "node:path";
import { defineConfig } from "vite";

// Shared pages inline assets so the uploaded file has nothing to fetch.
// U+FFFD is escaped because upload validators treat a literal replacement character as corruption.
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
