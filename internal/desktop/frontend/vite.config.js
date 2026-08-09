import { resolve } from "node:path";
import { defineConfig } from "vite";
import { svelte } from "@sveltejs/vite-plugin-svelte";

const page = (path) => resolve(import.meta.dirname, path);

// The palette is served by the workbench, so Vite cannot resolve it and drops
// the link silently if the source HTML carries one. Injected here it stays a
// blocking stylesheet in <head>, and crossorigin goes with it — the webview
// serves these pages from a scheme of its own.
const workbenchPages = {
  name: "qrouton-workbench-pages",
  transformIndexHtml: {
    order: "post",
    handler: (html) => ({
      html: html.replaceAll(" crossorigin", ""),
      tags: [{
        tag: "link",
        attrs: { rel: "stylesheet", href: "/tokens/colors.css" },
        injectTo: "head-prepend",
      }],
    }),
  },
};

export default defineConfig({
  plugins: [svelte(), workbenchPages],
  build: {
    // The tree Go embeds; emptying it keeps a deleted page out of the binary.
    outDir: "../assets",
    emptyOutDir: true,
    assetsDir: "bundle",
    rollupOptions: {
      // Served by the workbench; bundling it would pin a second version.
      external: [/^\/wails\//],
      input: {
        index: page("index.html"),
        terminal: page("terminal/index.html"),
        document: page("document/index.html"),
      },
    },
  },
});
