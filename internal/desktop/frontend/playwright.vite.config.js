import { resolve } from "node:path";
import { defineConfig } from "vite";
import { svelte } from "@sveltejs/vite-plugin-svelte";

export default defineConfig({
  plugins: [svelte()],
  optimizeDeps: { entries: ["tests/*.html"] },
  resolve: {
    alias: { "/wails/runtime.js": resolve(import.meta.dirname, "tests/wails-runtime.js") },
  },
});
